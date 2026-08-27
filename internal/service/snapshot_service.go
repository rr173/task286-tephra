package service

import (
	"fmt"

	"task286-tephra/internal/model"
	"task286-tephra/internal/snapshot"
)

// CreateSnapshot 从当前全部已裁决关系构建草稿快照。
func (s *Service) CreateSnapshot(name, note string) (*model.Snapshot, error) {
	corrs, err := s.store.ListCorrelations()
	if err != nil {
		return nil, err
	}
	builder := &snapshot.Builder{StandardVersion: s.standardizerVersion()}
	snap, err := builder.Build(name, note, corrs)
	if err != nil {
		return nil, err
	}
	return s.store.CreateSnapshot(snap)
}

// GetSnapshot 按 ID 读取快照（含链接行）。
func (s *Service) GetSnapshot(id string) (*model.Snapshot, error) {
	s.snapCacheMu.RLock()
	if cached, ok := s.snapCache[id]; ok {
		s.snapCacheMu.RUnlock()
		return cached, nil
	}
	s.snapCacheMu.RUnlock()
	snap, err := s.store.GetSnapshot(id)
	if err != nil {
		return nil, err
	}
	s.snapCacheMu.Lock()
	s.snapCache[id] = snap
	s.snapCacheMu.Unlock()
	return snap, nil
}

// ListSnapshots 列出全部快照。
func (s *Service) ListSnapshots() ([]model.Snapshot, error) {
	return s.store.ListSnapshots()
}

// PublishSnapshot 发布快照；发布后不可变，并固定参与批次的发布状态。
func (s *Service) PublishSnapshot(id string) (*model.Snapshot, error) {
	snap, err := s.store.GetSnapshot(id)
	if err != nil {
		return nil, err
	}
	if snap.Status != model.SnapDraft {
		return nil, fmt.Errorf("%w: snapshot %s is %s, not draft", model.ErrInvalidState, id, snap.Status)
	}
	return s.store.PublishSnapshot(id)
}

// SupersedeSnapshot 用新快照替代已发布旧快照。
func (s *Service) SupersedeSnapshot(oldID, newID string) (*model.Snapshot, error) {
	return s.store.SupersedeSnapshot(oldID, newID)
}

// DiffSnapshots 比较两个快照的关系差异。
func (s *Service) DiffSnapshots(leftID, rightID string) ([]snapshot.DiffEntry, error) {
	left, err := s.store.GetSnapshot(leftID)
	if err != nil {
		return nil, err
	}
	right, err := s.store.GetSnapshot(rightID)
	if err != nil {
		return nil, err
	}
	return snapshot.Diff(left, right), nil
}

// SealedSnapshots 返回全部已冻结（发布/替代）快照。
func (s *Service) SealedSnapshots() ([]model.Snapshot, error) {
	snaps, err := s.store.ListSnapshots()
	if err != nil {
		return nil, err
	}
	var out []model.Snapshot
	for _, sp := range snaps {
		if snapshot.Frozen(&sp) {
			out = append(out, sp)
		}
	}
	return out, nil
}
