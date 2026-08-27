// Package snapshot 实现相关快照的构建、发布与不可变语义。
//
// 快照是不可变的相关证据包：发布时固定标准化版本号、固定纳入的关系集合
// 与每条关系的距离/p 值/层位结果。发布后任何修改（改关系、改观测、加观测）
// 都不会影响已发布快照；新证据只会生成修订候选快照。
package snapshot

import (
	"fmt"
	"sort"

	"task286-tephra/internal/model"
)

// Builder 负责从候选关系集合构建快照内容。零值可用。
type Builder struct {
	// StandardVersion 是构建快照时使用的标准化版本。
	StandardVersion string
}

// Build 从已裁决的关系构造快照链接行。
// 仅纳入 confirmed（确认相关）与 rejected（否决）两种终态关系，
// 未裁决的候选不进入快照。
func (b *Builder) Build(name, note string, correlations []model.Correlation) (*model.Snapshot, error) {
	if b.StandardVersion == "" {
		return nil, fmt.Errorf("%w: standard version required", model.ErrInvalidInput)
	}
	links := make([]model.SnapshotLink, 0, len(correlations))
	for _, c := range correlations {
		if c.Status != model.CorrConfirmed && c.Status != model.CorrRejected {
			continue
		}
		links = append(links, model.SnapshotLink{
			CorrelationID: c.ID,
			UpperBatchID:  c.UpperBatchID,
			LowerBatchID:  c.LowerBatchID,
			Status:        string(c.Status),
			Distance:      c.Distance,
			PValue:        c.PValue,
			StratGap:      c.StratGap,
		})
	}
	if len(links) == 0 {
		return nil, fmt.Errorf("%w: snapshot requires at least one adjudicated correlation", model.ErrInvalidInput)
	}
	sort.Slice(links, func(i, j int) bool { return links[i].CorrelationID < links[j].CorrelationID })
	now := model.NowISO()
	return &model.Snapshot{
		ID:              "",
		Name:            name,
		Status:          model.SnapDraft,
		StandardVersion: b.StandardVersion,
		Note:            note,
		CreatedAt:       now,
		Links:           links,
	}, nil
}

// Publish 把草稿快照置为已发布并记录发布时间。
func Publish(s *model.Snapshot) error {
	if err := model.TransitionSnapshot(s.Status, model.SnapPublished); err != nil {
		return err
	}
	s.Status = model.SnapPublished
	s.PublishedAt = model.NowISO()
	return nil
}

// Supersede 用新快照替代旧快照。
func Supersede(old, newSnap *model.Snapshot) error {
	if old.Status != model.SnapPublished {
		return fmt.Errorf("%w: only published snapshots can be superseded", model.ErrInvalidState)
	}
	if newSnap.Status != model.SnapPublished {
		return fmt.Errorf("%w: replacement must be published first", model.ErrInvalidState)
	}
	old.Status = model.SnapSuperseded
	old.SupersededBy = newSnap.ID
	return nil
}

// Frozen 判断快照是否已不可变（发布或替代后）。
func Frozen(s *model.Snapshot) bool {
	return s.Status == model.SnapPublished || s.Status == model.SnapSuperseded
}
