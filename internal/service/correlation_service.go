package service

import (
	"fmt"

	"task286-tephra/internal/composition"
	"task286-tephra/internal/correlate"
	"task286-tephra/internal/fingerprint"
	"task286-tephra/internal/model"
	"task286-tephra/internal/stratigraphy"
)

// CreateCorrelation 创建两个灰层之间的相关候选（upper 在上、lower 在下），
// 并立即执行成分指纹 + 层位约束计算。
func (s *Service) CreateCorrelation(upperID, lowerID string) (*model.Correlation, error) {
	upper, err := s.store.GetBatch(upperID)
	if err != nil {
		return nil, err
	}
	lower, err := s.store.GetBatch(lowerID)
	if err != nil {
		return nil, err
	}
	if !model.BatchCanCompare(*upper) {
		return nil, fmt.Errorf("%w: batch %s status %s not comparable", model.ErrInvalidState, upperID, upper.Status)
	}
	if !model.BatchCanCompare(*lower) {
		return nil, fmt.Errorf("%w: batch %s status %s not comparable", model.ErrInvalidState, lowerID, lower.Status)
	}
	// 层位检查
	stratRes, err := stratigraphy.CheckBatches(upper, lower)
	if err != nil {
		return nil, err
	}
	// 指纹比较
	fp, err := s.computeFingerprint(upperID, lowerID)
	if err != nil {
		return nil, err
	}
	// 状态判定：成分相符且层位可行 → compatible；任一不满足 → 层位冲突态
	// （层位冲突态承载一切无法确认同源的情形，交由研究者裁决确认/否决）。
	status := model.CorrStratDiff
	if fp.Compatible && stratRes.OK {
		status = model.CorrCompatible
	}

	c := &model.Correlation{
		UpperBatchID:    upperID,
		LowerBatchID:    lowerID,
		Status:          status,
		Distance:        fp.Distance,
		DOF:             fp.DOF,
		PValue:          fp.PValue,
		Compatible:      fp.Compatible,
		StratOK:         stratRes.OK,
		StratGap:        stratRes.Gap,
		StratNote:       stratRes.Note,
		StandardVersion: s.standardizerVersion(),
	}
	return s.store.CreateCorrelation(c)
}

// computeFingerprint 计算两批次的指纹比较。
func (s *Service) computeFingerprint(upperID, lowerID string) (*fingerprint.Verdict, error) {
	upperObs, err := s.activeObservations(upperID)
	if err != nil {
		return nil, err
	}
	lowerObs, err := s.activeObservations(lowerID)
	if err != nil {
		return nil, err
	}
	// 指纹比较前对活跃观测统一做标准化（幂等：已标准化的直接使用）
	for i := range upperObs {
		if upperObs[i].Status == model.ObsRaw {
			norm, err := s.standardizer.Standardize(upperObs[i].Elements)
			if err != nil {
				return nil, err
			}
			upperObs[i].Elements = norm
			upperObs[i].Status = model.ObsNormalized
		}
	}
	for i := range lowerObs {
		if lowerObs[i].Status == model.ObsRaw {
			norm, err := s.standardizer.Standardize(lowerObs[i].Elements)
			if err != nil {
				return nil, err
			}
			lowerObs[i].Elements = norm
			lowerObs[i].Status = model.ObsNormalized
		}
	}
	upStats, err := composition.ComputeStatistics(upperObs)
	if err != nil {
		return nil, fmt.Errorf("upper batch: %w", err)
	}
	loStats, err := composition.ComputeStatistics(lowerObs)
	if err != nil {
		return nil, fmt.Errorf("lower batch: %w", err)
	}
	return fingerprint.Compare(upStats, loStats)
}

// activeObservations 返回批次内参与比较的标准化观测。
func (s *Service) activeObservations(batchID string) ([]model.Observation, error) {
	obs, err := s.store.ListObservationsByBatch(batchID)
	if err != nil {
		return nil, err
	}
	var out []model.Observation
	for _, o := range obs {
		if model.ObservationParticipates(o) {
			out = append(out, o)
		}
	}
	if len(out) < 2 {
		return nil, fmt.Errorf("%w: batch %s has %d active observations", model.ErrCorrNotReady, batchID, len(out))
	}
	return out, nil
}

// standardizerVersion 返回当前标准化版本。
func (s *Service) standardizerVersion() string {
	return normalizeVersion()
}

// GetCorrelation 按 ID 读取相关关系。
func (s *Service) GetCorrelation(id string) (*model.Correlation, error) {
	s.corrByIDMu.RLock()
	if cached, ok := s.corrByID[id]; ok {
		s.corrByIDMu.RUnlock()
		return cached, nil
	}
	s.corrByIDMu.RUnlock()
	c, err := s.store.GetCorrelation(id)
	if err != nil {
		return nil, err
	}
	s.corrByIDMu.Lock()
	s.corrByID[id] = c
	s.corrByIDMu.Unlock()
	return c, nil
}

// ListCorrelations 列出全部相关关系。
func (s *Service) ListCorrelations() ([]model.Correlation, error) {
	return s.store.ListCorrelations()
}

// ListCorrelationsByBatch 列出批次参与的相关关系。
func (s *Service) ListCorrelationsByBatch(batchID string) ([]model.Correlation, error) {
	return s.store.ListCorrelationsByBatch(batchID)
}

// RecomputeCorrelation 用当前观测与标准化版本重新计算关系。
// 已裁决关系必须重置为候选后才能重算；已入冻结快照的关系拒绝重算。
func (s *Service) RecomputeCorrelation(id string) (*model.Correlation, error) {
	frozen, err := s.store.IsCorrelationFrozenInSnapshot(id)
	if err != nil {
		return nil, err
	}
	if frozen {
		return nil, fmt.Errorf("%w: correlation %s is sealed inside a published snapshot", model.ErrSealed, id)
	}
	c, err := s.store.GetCorrelation(id)
	if err != nil {
		return nil, err
	}
	if err := correlate.ReopenForRecompute(c); err != nil {
		return nil, err
	}
	upper, err := s.store.GetBatch(c.UpperBatchID)
	if err != nil {
		return nil, err
	}
	lower, err := s.store.GetBatch(c.LowerBatchID)
	if err != nil {
		return nil, err
	}
	stratRes, err := stratigraphy.CheckBatches(upper, lower)
	if err != nil {
		return nil, err
	}
	fp, err := s.computeFingerprint(c.UpperBatchID, c.LowerBatchID)
	if err != nil {
		return nil, err
	}
	c.Status = model.CorrStratDiff
	c.Distance = fp.Distance
	c.DOF = fp.DOF
	c.PValue = fp.PValue
	c.Compatible = fp.Compatible
	c.StratOK = stratRes.OK
	c.StratGap = stratRes.Gap
	c.StratNote = stratRes.Note
	c.StandardVersion = s.standardizerVersion()
	if fp.Compatible && stratRes.OK {
		c.Status = model.CorrCompatible
	}
	if err := s.store.SaveCorrelationResult(c); err != nil {
		return nil, err
	}
	return c, nil
}

// AdjudicateCorrelation 裁决相关关系。
func (s *Service) AdjudicateCorrelation(id, verdict, who, note string) (*model.Correlation, error) {
	c, err := s.store.GetCorrelation(id)
	if err != nil {
		return nil, err
	}
	if who == "" {
		who = "anonymous"
	}
	if err := correlate.Adjudicate(c, verdict, who, note); err != nil {
		return nil, err
	}
	if err := s.store.AdjudicateCorrelation(c); err != nil {
		return nil, err
	}
	return c, nil
}

// RankCorrelations 返回全部关系的加权排名（候选优先展示）。
func (s *Service) RankCorrelations() ([]correlate.Score, error) {
	corrs, err := s.store.ListCorrelations()
	if err != nil {
		return nil, err
	}
	var scores []correlate.Score
	for _, c := range corrs {
		scores = append(scores, correlate.ComputeScore(
			c.ID, c.UpperBatchID, c.LowerBatchID, c.Distance, c.PValue, c.Compatible, c.StratOK, c.DOF))
	}
	return correlate.RankCorrelations(scores), nil
}
