package service

import (
	"testing"

	"task286-tephra/internal/model"
	"task286-tephra/internal/store"
)

// testService 构造使用内存数据库的测试服务。
func testService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open("")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st)
}

// obs 构造一条玻璃成分观测。
func obs(sampleNo string, k2o, depthShift float64) model.Observation {
	_ = depthShift
	return model.Observation{
		SampleNo:   sampleNo,
		GrainCount: 8,
		Elements: map[string]model.ElementValue{
			"SiO2": {Value: 72.4, Error: 0.4}, "TiO2": {Value: 0.28, Error: 0.05},
			"Al2O3": {Value: 12.6, Error: 0.2}, "FeO": {Value: 2.9, Error: 0.15},
			"MnO": {Value: 0.08, Error: 0.04}, "MgO": {Value: 0.35, Error: 0.05},
			"CaO": {Value: 1.55, Error: 0.1}, "Na2O": {Value: 4.1, Error: 0.2},
			"K2O": {Value: 3.1 + k2o, Error: 0.15}, "P2O5": {Value: 0.06, Error: 0.03},
		},
	}
}

func TestEndToEndCorrelationAndSnapshot(t *testing.T) {
	svc := testService(t)

	// 1. 创建三个批次
	upper, err := svc.CreateBatch(&model.Batch{Name: "KR-03", Site: "Krakatoa", Formation: "A", TopDepth: 10.0, BottomDepth: 10.8})
	if err != nil {
		t.Fatalf("create upper: %v", err)
	}
	lower, err := svc.CreateBatch(&model.Batch{Name: "KR-04", Site: "Krakatoa", Formation: "A", TopDepth: 11.0, BottomDepth: 11.9})
	if err != nil {
		t.Fatalf("create lower: %v", err)
	}
	far, err := svc.CreateBatch(&model.Batch{Name: "KR-09", Site: "Krakatoa", Formation: "D", TopDepth: 80.0, BottomDepth: 80.7})
	if err != nil {
		t.Fatalf("create far: %v", err)
	}

	// 2. 导入观测（同源），重复导入应幂等
	for _, o := range []model.Observation{obs("G-01", 0, 0), obs("G-02", 0.1, 0), obs("G-03", -0.1, 0)} {
		o.BatchID = upper.ID
		_, skipped, err := svc.ImportObservation(&o)
		if err != nil || skipped {
			t.Fatalf("import upper: %v skipped=%v", err, skipped)
		}
	}
	dup := obs("G-01", 0, 0)
	dup.BatchID = upper.ID
	if _, skipped, err := svc.ImportObservation(&dup); err != nil || !skipped {
		t.Fatalf("duplicate import should skip: err=%v skipped=%v", err, skipped)
	}
	for _, o := range []model.Observation{obs("G-04", 0, 0), obs("G-05", 0.05, 0), obs("G-06", -0.05, 0)} {
		o.BatchID = lower.ID
		if _, _, err := svc.ImportObservation(&o); err != nil {
			t.Fatalf("import lower: %v", err)
		}
	}
	for _, o := range []model.Observation{obs("G-07", 0, 0), obs("G-08", 0.05, 0), obs("G-09", -0.05, 0)} {
		o.BatchID = far.ID
		if _, _, err := svc.ImportObservation(&o); err != nil {
			t.Fatalf("import far: %v", err)
		}
	}

	// 3. 非法观测被拒绝
	bad := obs("G-X", 0, 0)
	bad.BatchID = upper.ID
	bad.Elements["UO2"] = model.ElementValue{Value: 1, Error: 0.1}
	if _, _, err := svc.ImportObservation(&bad); err == nil {
		t.Fatal("unknown element should be rejected")
	}

	// 4. 标准化
	for _, id := range []string{upper.ID, lower.ID, far.ID} {
		n, warns, err := svc.StandardizeBatch(id)
		if err != nil || n != 3 || len(warns) != 0 {
			t.Fatalf("standardize %s: n=%d warns=%v err=%v", id, n, warns, err)
		}
	}

	// 5. 批次置为待比较
	for _, id := range []string{upper.ID, lower.ID, far.ID} {
		if _, err := svc.TransitionBatch(id, "ready"); err != nil {
			t.Fatalf("ready %s: %v", id, err)
		}
	}

	// 6. 相关候选：upper~lower 应成分相符且层位可行
	c, err := svc.CreateCorrelation(upper.ID, lower.ID)
	if err != nil {
		t.Fatalf("create correlation: %v", err)
	}
	if c.Status != model.CorrCompatible {
		t.Fatalf("upper~lower should be compatible, got %s", c.Status)
	}
	if !c.Compatible || !c.StratOK {
		t.Fatalf("compatible=%v stratOK=%v", c.Compatible, c.StratOK)
	}
	if c.Distance < 0 || c.DOF != 10 {
		t.Errorf("distance=%v dof=%d", c.Distance, c.DOF)
	}

	// 7. 层位不可能的候选：lower~far 应层位冲突（相距过远）
	c2, err := svc.CreateCorrelation(lower.ID, far.ID)
	if err != nil {
		t.Fatalf("create bad correlation: %v", err)
	}
	if c2.Status != model.CorrStratDiff {
		t.Fatalf("lower~far should be stratigraphic_conflict, got %s", c2.Status)
	}
	if c2.StratOK {
		t.Fatal("lower~far should not be stratigraphically OK")
	}

	// 8. 裁决
	if _, err := svc.AdjudicateCorrelation(c.ID, "confirmed", "researcher-a", "e2e"); err != nil {
		t.Fatalf("adjudicate: %v", err)
	}
	if _, err := svc.AdjudicateCorrelation(c2.ID, "rejected", "researcher-a", "e2e"); err != nil {
		t.Fatalf("adjudicate reject: %v", err)
	}
	// 已裁决的关系拒绝再次裁决
	if _, err := svc.AdjudicateCorrelation(c.ID, "rejected", "x", "again"); err == nil {
		t.Fatal("adjudicated correlation should not be re-adjudicated")
	}

	// 9. 快照
	snap, err := svc.CreateSnapshot("snap-1", "e2e")
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if len(snap.Links) != 2 {
		t.Fatalf("snapshot links = %d, want 2", len(snap.Links))
	}
	published, err := svc.PublishSnapshot(snap.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Status != model.SnapPublished {
		t.Fatalf("status = %s", published.Status)
	}
	// 重复发布被拒绝
	if _, err := svc.PublishSnapshot(snap.ID); err == nil {
		t.Fatal("double publish should fail")
	}
	// 已发布快照关系不可重算
	if _, err := svc.RecomputeCorrelation(c.ID); err == nil {
		t.Fatal("correlation inside published snapshot should not recompute")
	}

	// 10. 自检
	issues, err := svc.SelfCheck()
	if err != nil || len(issues) > 0 {
		t.Fatalf("selfcheck: %v %v", err, issues)
	}
}

func TestPersistenceRestartRecovery(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/tephra-test.db"

	// 第一轮写入
	st1, err := store.Open(path)
	if err != nil {
		t.Fatalf("open round1: %v", err)
	}
	svc1 := New(st1)
	b, err := svc1.CreateBatch(&model.Batch{Name: "KB-01", TopDepth: 1.0, BottomDepth: 1.5})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, o := range []model.Observation{obs("A", 0, 0), obs("B", 0.1, 0), obs("C", -0.1, 0)} {
		o.BatchID = b.ID
		if _, _, err := svc1.ImportObservation(&o); err != nil {
			t.Fatalf("import: %v", err)
		}
	}
	if _, _, err := svc1.StandardizeBatch(b.ID); err != nil {
		t.Fatalf("standardize: %v", err)
	}
	if _, err := svc1.TransitionBatch(b.ID, "ready"); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := st1.Close(); err != nil {
		t.Fatalf("close round1: %v", err)
	}

	// 第二轮重开验证恢复
	st2, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()
	svc2 := New(st2)
	reloaded, err := svc2.GetBatch(b.ID)
	if err != nil {
		t.Fatalf("reload batch: %v", err)
	}
	if reloaded.Status != model.BatchReady {
		t.Fatalf("status = %s, want ready", reloaded.Status)
	}
	obsList, err := svc2.Store().ListObservationsByBatch(b.ID)
	if err != nil {
		t.Fatalf("list obs: %v", err)
	}
	if len(obsList) != 3 {
		t.Fatalf("observations = %d, want 3", len(obsList))
	}
	// 重建统计应成功（重启后数据完整可用于指纹计算）
	active := []model.Observation{}
	for _, o := range obsList {
		if model.ObservationParticipates(o) {
			active = append(active, o)
		}
	}
	if len(active) != 3 {
		t.Fatalf("active = %d, want 3", len(active))
	}
}
