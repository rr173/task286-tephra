// Package service 是火山灰层玻璃成分对比服务的业务编排层。
//
// 每个方法编排 store 与领域包（composition/normalize/fingerprint/
// stratigraphy/redeposition/correlate/snapshot），对外暴露完整业务操作。
package service

import (
	"fmt"
	"sync"

	"task286-tephra/internal/model"
	"task286-tephra/internal/normalize"
	"task286-tephra/internal/store"
)

// Service 聚合数据访问与领域编排。
type Service struct {
	store        *store.Store
	standardizer *normalize.Standardizer

	snapCacheMu sync.RWMutex
	snapCache   map[string]*model.Snapshot
}

// New 构造服务。
func New(st *store.Store) *Service {
	return &Service{store: st, standardizer: normalize.NewStandardizer(), snapCache: map[string]*model.Snapshot{}}
}

// Store 暴露底层 store（供测试与内部扩展）。
func (s *Service) Store() *store.Store {
	return s.store
}

// Standardizer 暴露标准化器（供自检与工具方法）。
func (s *Service) Standardizer() *normalize.Standardizer {
	return s.standardizer
}

// normalizeVersion 返回当前标准化算法版本。
func normalizeVersion() string {
	return normalize.Version
}

// SelfCheck 对指定数据库执行一致性自检：
//  1. 数据库连通且迁移完成；
//  2. 全部批次/观测/关系/快照可读；
//  3. 已发布快照的关系均为终态（confirmed/rejected）且行数一致。
//
// 返回问题列表；空列表表示通过。自检不修改任何数据。
func (s *Service) SelfCheck() ([]string, error) {
	var issues []string
	batches, err := s.store.ListBatches()
	if err != nil {
		return nil, fmt.Errorf("selfcheck list batches: %w", err)
	}
	for _, b := range batches {
		if b.TopDepth < 0 || b.BottomDepth <= b.TopDepth {
			issues = append(issues, fmt.Sprintf("batch %s has invalid depth range [%.3f, %.3f]", b.ID, b.TopDepth, b.BottomDepth))
		}
		if _, err := s.store.ListObservationsByBatch(b.ID); err != nil {
			issues = append(issues, fmt.Sprintf("batch %s observations unreadable: %v", b.ID, err))
		}
	}
	corrs, err := s.store.ListCorrelations()
	if err != nil {
		return nil, fmt.Errorf("selfcheck list correlations: %w", err)
	}
	for _, c := range corrs {
		if c.UpperBatchID == c.LowerBatchID {
			issues = append(issues, fmt.Sprintf("correlation %s correlates batch with itself", c.ID))
		}
		if (c.Status == model.CorrConfirmed || c.Status == model.CorrRejected) && c.Verdict == "" {
			issues = append(issues, fmt.Sprintf("correlation %s adjudicated without verdict actor", c.ID))
		}
	}
	snaps, err := s.store.ListSnapshots()
	if err != nil {
		return nil, fmt.Errorf("selfcheck list snapshots: %w", err)
	}
	for _, snap := range snaps {
		full, err := s.store.GetSnapshot(snap.ID)
		if err != nil {
			issues = append(issues, fmt.Sprintf("snapshot %s unreadable: %v", snap.ID, err))
			continue
		}
		if full.Status == model.SnapPublished || full.Status == model.SnapSuperseded {
			for _, l := range full.Links {
				if l.Status != string(model.CorrConfirmed) && l.Status != string(model.CorrRejected) {
					issues = append(issues, fmt.Sprintf("snapshot %s contains non-terminal correlation %s", snap.ID, l.CorrelationID))
				}
			}
		}
	}
	return issues, nil
}
