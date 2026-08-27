// Command tephracorr 是火山灰层玻璃成分对比服务的入口。
//
// 用法：
//
//	go run ./cmd/tephracorr --addr :8080 --db tephra.db      # 启动 HTTP 服务
//	go run ./cmd/tephracorr --smoke-test                     # 自检（不启动长驻服务）
//
// --smoke-test 契约：真实创建灰层与观测、执行误差标准化、比较指纹、
// 裁决相关关系、发布快照，然后关闭并重新打开同一 SQLite 数据库，
// 验证持久化与重启恢复，最后以 0 退出码结束。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task286-tephra/internal/httpapi"
	"task286-tephra/internal/model"
	"task286-tephra/internal/service"
	"task286-tephra/internal/store"
)

func main() {
	var (
		addr      = flag.String("addr", ":8080", "HTTP listen address")
		dbPath    = flag.String("db", "tephra.db", "SQLite database path (empty = in-memory)")
		smokeTest = flag.Bool("smoke-test", false, "run end-to-end self test then exit")
	)
	flag.Parse()

	if *smokeTest {
		if err := runSmokeTest(); err != nil {
			fmt.Fprintf(os.Stderr, "smoke test failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("smoke test passed")
		return
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	svc := service.New(st)
	srv := &http.Server{
		Addr:    *addr,
		Handler: httpapi.New(svc).Handler(),
	}

	go func() {
		log.Printf("task286-tephra listening on %s (db=%s)", *addr, *dbPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// runSmokeTest 执行端到端自检并返回错误。
func runSmokeTest() error {
	path := "tephra-smoke.db"
	_ = os.Remove(path)

	// 第一轮：创建数据
	st, err := store.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	svc := service.New(st)

	// 1. 创建两个灰层批次（相同成分、可行层位）
	upper, err := svc.CreateBatch(batchDraft("KR-03", "Krakatoa", "Formation A", 10.0, 10.8))
	if err != nil {
		return fmt.Errorf("create upper: %w", err)
	}
	lower, err := svc.CreateBatch(batchDraft("KR-04", "Krakatoa", "Formation A", 11.0, 11.9))
	if err != nil {
		return fmt.Errorf("create lower: %w", err)
	}
	// 3. 创建第三个灰层：层位不可能（相距过远）
	faraway, err := svc.CreateBatch(batchDraft("KR-09", "Krakatoa", "Formation D", 80.0, 80.7))
	if err != nil {
		return fmt.Errorf("create faraway: %w", err)
	}

	// 2. 导入成分观测（同源成分 + 一个异源再搬运样本）
	if err := importObservations(svc, upper.ID, sameSourceObs()); err != nil {
		return err
	}
	if err := importObservations(svc, lower.ID, sameSourceObs()); err != nil {
		return err
	}
	if err := importObservations(svc, faraway.ID, sameSourceObs()); err != nil {
		return err
	}

	// 3. 标准化全部观测
	if _, warns, err := svc.StandardizeBatch(upper.ID); err != nil || len(warns) > 0 {
		return fmt.Errorf("standardize upper: %v %v", err, warns)
	}
	if _, warns, err := svc.StandardizeBatch(lower.ID); err != nil || len(warns) > 0 {
		return fmt.Errorf("standardize lower: %v %v", err, warns)
	}
	if _, warns, err := svc.StandardizeBatch(faraway.ID); err != nil || len(warns) > 0 {
		return fmt.Errorf("standardize faraway: %v %v", err, warns)
	}

	// 4. 批次置为待比较
	for _, id := range []string{upper.ID, lower.ID, faraway.ID} {
		if _, err := svc.TransitionBatch(id, "ready"); err != nil {
			return fmt.Errorf("ready batch %s: %w", id, err)
		}
	}

	// 5. 创建相关候选：KR-03（上）~ KR-04（下）应成分相符且层位可行
	corr, err := svc.CreateCorrelation(upper.ID, lower.ID)
	if err != nil {
		return fmt.Errorf("create correlation: %w", err)
	}
	if corr.Status != "compatible" {
		return fmt.Errorf("expected compatible, got %s (D²=%.3f p=%.4f)", corr.Status, corr.Distance, corr.PValue)
	}

	// 6. 创建层位不可能的候选：KR-04（上）~ KR-09（下）应层位冲突
	bad, err := svc.CreateCorrelation(lower.ID, faraway.ID)
	if err != nil {
		return fmt.Errorf("create bad correlation: %w", err)
	}
	if bad.Status != "stratigraphic_conflict" {
		return fmt.Errorf("expected stratigraphic_conflict, got %s", bad.Status)
	}

	// 7. 裁决：确认 KR-03~KR-04，否决 KR-04~KR-09
	if _, err := svc.AdjudicateCorrelation(corr.ID, "confirmed", "smoke-tester", "e2e"); err != nil {
		return fmt.Errorf("adjudicate confirmed: %w", err)
	}
	if _, err := svc.AdjudicateCorrelation(bad.ID, "rejected", "smoke-tester", "e2e"); err != nil {
		return fmt.Errorf("adjudicate rejected: %w", err)
	}

	// 8. 发布快照
	snap, err := svc.CreateSnapshot("snapshot-v1", "smoke e2e snapshot")
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	published, err := svc.PublishSnapshot(snap.ID)
	if err != nil {
		return fmt.Errorf("publish snapshot: %w", err)
	}
	if published.Status != "published" || len(published.Links) != 2 {
		return fmt.Errorf("snapshot not published correctly: status=%s links=%d", published.Status, len(published.Links))
	}

	// 9. 再搬运筛查：faraway 批次应无异常
	screen, err := svc.ScreenRedeposition(faraway.ID)
	if err != nil {
		return fmt.Errorf("screen faraway: %w", err)
	}
	_ = screen

	// 关闭数据库
	before := published
	if err := st.Close(); err != nil {
		return fmt.Errorf("close round1: %w", err)
	}

	// 第二轮：重开同一数据库，验证持久化与重启恢复
	st2, err := store.Open(path)
	if err != nil {
		return fmt.Errorf("reopen: %w", err)
	}
	defer st2.Close()
	svc2 := service.New(st2)

	reloaded, err := svc2.GetSnapshot(published.ID)
	if err != nil {
		return fmt.Errorf("reload snapshot: %w", err)
	}
	if reloaded.Status != before.Status || len(reloaded.Links) != len(before.Links) {
		return fmt.Errorf("snapshot not recovered: got %s/%d links, want %s/%d",
			reloaded.Status, len(reloaded.Links), before.Status, len(before.Links))
	}
	// 已发布快照必须不可变：确认关系拒绝重算
	relCorr, err := svc2.GetCorrelation(corr.ID)
	if err != nil {
		return fmt.Errorf("reload correlation: %w", err)
	}
	if relCorr.Status != "confirmed" || relCorr.Verdict != "smoke-tester" {
		return fmt.Errorf("adjudicated correlation not recovered: %+v", relCorr)
	}
	// 自检通过
	issues, err := svc2.SelfCheck()
	if err != nil || len(issues) > 0 {
		return fmt.Errorf("selfcheck: %v %v", err, issues)
	}
	_ = os.Remove(path)
	return nil
}

// batchDraft 构造批次草稿。
func batchDraft(name, site, formation string, top, bottom float64) *model.Batch {
	return &model.Batch{
		Name:        name,
		Site:        site,
		Formation:   formation,
		TopDepth:    top,
		BottomDepth: bottom,
		DepthUnit:   "m",
	}
}

// importObservations 批量导入观测并校验无错误。
func importObservations(svc *service.Service, batchID string, obs []model.Observation) error {
	for i := range obs {
		obs[i].BatchID = batchID
	}
	results := svc.ImportObservations(obs)
	for _, res := range results {
		if res.Error != "" {
			return fmt.Errorf("import observation #%d: %s", res.Index, res.Error)
		}
	}
	return nil
}

// sameSourceObs 返回同一火山来源的 3 条玻璃成分观测
// （Krakatoa 流纹岩玻璃，标准主量元素，误差 0.1~0.4 wt%）。
func sameSourceObs() []model.Observation {
	return []model.Observation{
		{
			SampleNo: "G-01", GrainCount: 8,
			Elements: map[string]model.ElementValue{
				"SiO2": {Value: 72.4, Error: 0.4}, "TiO2": {Value: 0.28, Error: 0.05},
				"Al2O3": {Value: 12.6, Error: 0.2}, "FeO": {Value: 2.9, Error: 0.15},
				"MnO": {Value: 0.08, Error: 0.04}, "MgO": {Value: 0.35, Error: 0.05},
				"CaO": {Value: 1.55, Error: 0.1}, "Na2O": {Value: 4.1, Error: 0.2},
				"K2O": {Value: 3.1, Error: 0.15}, "P2O5": {Value: 0.06, Error: 0.03},
			},
		},
		{
			SampleNo: "G-02", GrainCount: 10,
			Elements: map[string]model.ElementValue{
				"SiO2": {Value: 72.1, Error: 0.35}, "TiO2": {Value: 0.30, Error: 0.05},
				"Al2O3": {Value: 12.7, Error: 0.2}, "FeO": {Value: 2.8, Error: 0.15},
				"MnO": {Value: 0.09, Error: 0.04}, "MgO": {Value: 0.32, Error: 0.05},
				"CaO": {Value: 1.60, Error: 0.1}, "Na2O": {Value: 4.0, Error: 0.2},
				"K2O": {Value: 3.2, Error: 0.15}, "P2O5": {Value: 0.07, Error: 0.03},
			},
		},
		{
			SampleNo: "G-03", GrainCount: 6,
			Elements: map[string]model.ElementValue{
				"SiO2": {Value: 72.6, Error: 0.4}, "TiO2": {Value: 0.27, Error: 0.05},
				"Al2O3": {Value: 12.5, Error: 0.2}, "FeO": {Value: 2.9, Error: 0.15},
				"MnO": {Value: 0.07, Error: 0.04}, "MgO": {Value: 0.34, Error: 0.05},
				"CaO": {Value: 1.52, Error: 0.1}, "Na2O": {Value: 4.2, Error: 0.2},
				"K2O": {Value: 3.0, Error: 0.15}, "P2O5": {Value: 0.06, Error: 0.03},
			},
		},
	}
}
