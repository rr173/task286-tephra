package service

import (
	"fmt"

	"task286-tephra/internal/composition"
	"task286-tephra/internal/model"
	"task286-tephra/internal/redeposition"
)

// ImportObservation 导入一条成分观测（幂等）。
//   - 校验元素白名单、单位与误差；
//   - 计算内容指纹，重复导入返回 skipped=true；
//   - 批次封存（sealed）时拒绝导入。
func (s *Service) ImportObservation(o *model.Observation) (*model.Observation, bool, error) {
	batch, err := s.store.GetBatch(o.BatchID)
	if err != nil {
		return nil, false, err
	}
	if batch.Status == model.BatchSealed {
		return nil, false, fmt.Errorf("import: %v", model.ErrSealed)
	}
	if err := composition.ValidateElements(o.Elements); err != nil {
		return nil, false, err
	}
	// 指纹（带批次与样品号）
	o.ContentHash = composition.ContentHash(o.BatchID, o.SampleNo, o.Unit, o.Elements)
	if o.GrainCount <= 0 {
		o.GrainCount = 1
	}
	return s.store.CreateObservation(o)
}

// ImportObservations 批量导入观测，返回每条结果。
func (s *Service) ImportObservations(obs []model.Observation) []ImportResult {
	results := make([]ImportResult, 0, len(obs))
	for i := range obs {
		created, skipped, err := s.ImportObservation(&obs[i])
		res := ImportResult{Index: i, Skipped: skipped}
		if err != nil {
			res.Error = err.Error()
		} else {
			res.ID = created.ID
		}
		results = append(results, res)
	}
	return results
}

// ImportResult 描述批量导入中单条的结果。
type ImportResult struct {
	Index   int    `json:"index"`
	ID      string `json:"id,omitempty"`
	Skipped bool   `json:"skipped"`
	Error   string `json:"error,omitempty"`
}

// StandardizeObservation 对单条观测执行误差标准化。
func (s *Service) StandardizeObservation(id string) (*model.Observation, error) {
	o, err := s.store.GetObservation(id)
	if err != nil {
		return nil, err
	}
	if o.Status != model.ObsRaw {
		return nil, fmt.Errorf("%w: observation %s already standardized", model.ErrInvalidState, id)
	}
	norm, err := s.standardizer.Standardize(o.Elements)
	if err != nil {
		return nil, err
	}
	if err := s.store.SaveNormalizedElements(id, norm); err != nil {
		return nil, err
	}
	return s.store.GetObservation(id)
}

// StandardizeBatch 批量标准化批次内全部 raw 观测。
// 返回成功条数与失败明细；观测数不足或质量差时给出诊断。
func (s *Service) StandardizeBatch(batchID string) (int, []string, error) {
	batch, err := s.store.GetBatch(batchID)
	if err != nil {
		return 0, nil, err
	}
	if batch.Status == model.BatchSealed {
		return 0, nil, fmt.Errorf("%w: batch %s is sealed", model.ErrSealed, batchID)
	}
	obs, err := s.store.ListObservationsByBatch(batchID)
	if err != nil {
		return 0, nil, err
	}
	ok := 0
	var warns []string
	for i := range obs {
		if obs[i].Status != model.ObsRaw {
			continue
		}
		q := s.standardizer.AssessQuality(obs[i].Elements)
		if !q.Usable {
			warns = append(warns, fmt.Sprintf("observation %s (%s): %s", obs[i].ID, obs[i].SampleNo, q.Reason))
		}
		norm, err := s.standardizer.Standardize(obs[i].Elements)
		if err != nil {
			warns = append(warns, fmt.Sprintf("observation %s: %v", obs[i].ID, err))
			continue
		}
		if err := s.store.SaveNormalizedElements(obs[i].ID, norm); err != nil {
			warns = append(warns, fmt.Sprintf("observation %s: %v", obs[i].ID, err))
			continue
		}
		ok++
	}
	return ok, warns, nil
}

// ScreenRedeposition 对批次执行再搬运筛查；发现离群时自动把批次置为需复核。
func (s *Service) ScreenRedeposition(batchID string) (*redeposition.Result, error) {
	batch, err := s.store.GetBatch(batchID)
	if err != nil {
		return nil, err
	}
	obs, err := s.store.ListObservationsByBatch(batchID)
	if err != nil {
		return nil, err
	}
	var active []model.Observation
	for _, o := range obs {
		if o.Status == model.ObsNormalized || o.Status == model.ObsRaw {
			active = append(active, o)
		}
	}
	res := redeposition.Detect(batchID, active)
	if len(res.Outliers) > 0 && batch.Status == model.BatchReady {
		_ = s.store.SetBatchReview(batchID)
	}
	return &res, nil
}

// MarkRedeposited 把观测标记为再搬运（进入排除候选，状态机校验）。
func (s *Service) MarkRedeposited(id, note string) (*model.Observation, error) {
	o, err := s.store.GetObservation(id)
	if err != nil {
		return nil, err
	}
	if o.Status == model.ObsExcluded {
		return nil, fmt.Errorf("%w: observation %s already excluded", model.ErrInvalidState, id)
	}
	return s.store.UpdateObservationStatus(id, model.ObsRedeposited, note)
}

// ExcludeObservation 排除观测（不参与比较）。
func (s *Service) ExcludeObservation(id, note string) (*model.Observation, error) {
	o, err := s.store.GetObservation(id)
	if err != nil {
		return nil, err
	}
	if o.Status == model.ObsExcluded {
		return nil, fmt.Errorf("%w: observation %s already excluded", model.ErrInvalidState, id)
	}
	return s.store.UpdateObservationStatus(id, model.ObsExcluded, note)
}

// RestoreObservation 恢复被排除/再搬运的观测（回 normalized）。
func (s *Service) RestoreObservation(id, note string) (*model.Observation, error) {
	return s.store.UpdateObservationStatus(id, model.ObsNormalized, note)
}
