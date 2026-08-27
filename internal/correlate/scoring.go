package correlate

import (
	"sort"

	"task286-tephra/internal/fingerprint"
)

// Score 描述一个相关候选的综合评分（用于排序与筛选）。
type Score struct {
	CorrelationID string                 `json:"correlation_id"`
	UpperBatchID  string                 `json:"upper_batch_id"`
	LowerBatchID  string                 `json:"lower_batch_id"`
	Distance      float64                `json:"distance"`
	PValue        float64                `json:"p_value"`
	Compatible    bool                   `json:"compatible"`
	StratOK       bool                   `json:"strat_ok"`
	Rank          fingerprint.DistanceRank `json:"rank"`
	// Weighted 是加权分数：距离越小、层位越可行，分数越高（0~1）。
	// score = 0.7*compat + 0.3*strat，其中 compat = p 值经 sigmoid 映射。
	Weighted float64 `json:"weighted"`
}

// ComputeScore 计算单个候选的加权分数。
func ComputeScore(correlationID, upperID, lowerID string, distance, pValue float64, compatible, stratOK bool, dof int) Score {
	s := Score{
		CorrelationID: correlationID,
		UpperBatchID:  upperID,
		LowerBatchID:  lowerID,
		Distance:      distance,
		PValue:        pValue,
		Compatible:    compatible,
		StratOK:       stratOK,
		Rank:          fingerprint.RankDistance(distance, dof),
	}
	// 把 p 值映射到 [0,1] 的相容度：p=0 → 0，p=alpha(0.05) → 0.5，p=1 → 1
	compatScore := pValue / (pValue + 0.05)
	compatScore = compatScore / (compatScore + 1) * 2 // 单调映射到 (0,1)
	if !compatible {
		compatScore = 0
	}
	stratScore := 0.0
	if stratOK {
		stratScore = 1.0
	}
	s.Weighted = 0.7*compatScore + 0.3*stratScore
	return s
}

// RankCorrelations 对候选列表按加权分数降序排序。
func RankCorrelations(scores []Score) []Score {
	out := make([]Score, len(scores))
	copy(out, scores)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Weighted != out[j].Weighted {
			return out[i].Weighted > out[j].Weighted
		}
		return out[i].Distance < out[j].Distance
	})
	return out
}
