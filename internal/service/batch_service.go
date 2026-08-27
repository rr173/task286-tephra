package service

import (
	"fmt"

	"task286-tephra/internal/model"
)

// CreateBatch 创建灰层批次。名称必须唯一；层位区间必须合法。
func (s *Service) CreateBatch(b *model.Batch) (*model.Batch, error) {
	if b.Name == "" {
		return nil, fmt.Errorf("%w: batch name required", model.ErrInvalidInput)
	}
	if !model.ValidDepthRange(b.TopDepth, b.BottomDepth) {
		return nil, fmt.Errorf("%w: top=%.3f bottom=%.3f", model.ErrInvertedRange, b.TopDepth, b.BottomDepth)
	}
	if b.DepthUnit == "" {
		b.DepthUnit = "m"
	}
	if err := s.store.CreateBatch(b); err != nil {
		return nil, err
	}
	return b, nil
}

// GetBatch 按 ID 读取批次。
func (s *Service) GetBatch(id string) (*model.Batch, error) {
	return s.store.GetBatch(id)
}

// ListBatches 列出全部批次。
func (s *Service) ListBatches() ([]model.Batch, error) {
	return s.store.ListBatches()
}

// TransitionBatch 执行批次状态流转。
func (s *Service) TransitionBatch(id string, to model.BatchStatus) (*model.Batch, error) {
	b, err := s.store.GetBatch(id)
	if err != nil {
		return nil, err
	}
	if err := model.TransitionBatch(b.Status, to); err != nil {
		return nil, err
	}
	// ready 状态要求至少 2 条活跃观测
	if to == model.BatchReady {
		cnt, err := s.store.CountActiveObservations(id)
		if err != nil {
			return nil, err
		}
		if cnt < 2 {
			return nil, fmt.Errorf("%w: batch %s needs at least 2 active observations, got %d", model.ErrInvalidState, id, cnt)
		}
	}
	return s.store.UpdateBatchStatus(id, to)
}

// SealBatch 封存批次（终态，禁止任何修改）。
func (s *Service) SealBatch(id string) (*model.Batch, error) {
	return s.TransitionBatch(id, model.BatchSealed)
}
