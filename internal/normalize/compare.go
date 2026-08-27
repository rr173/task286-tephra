package normalize

import (
	"math"

	"task286-tephra/internal/model"
)

// CompareResult 描述两个标准化观测之间的逐元素标准化差异。
type CompareResult struct {
	Element   string  `json:"element"`
	Diff      float64 `json:"diff"`      // 含量差 a-b
	Z         float64 `json:"z"`         // 标准化差异
	Significant bool  `json:"significant"` // |z|>2
}

// CompareObservations 逐元素比较两个标准化观测。
func CompareObservations(a, b map[string]model.ElementValue) []CompareResult {
	var out []CompareResult
	for symbol, av := range a {
		bv, ok := b[symbol]
		if !ok {
			continue
		}
		z := StandardizedDifference(av, bv)
		out = append(out, CompareResult{
			Element:     symbol,
			Diff:        av.Value - bv.Value,
			Z:           z,
			Significant: math.Abs(z) > 2,
		})
	}
	return out
}

// MaxAbsZ 返回最大 |z| 及其元素（用于快速告警）。
func MaxAbsZ(results []CompareResult) (string, float64) {
	bestSymbol := ""
	bestZ := 0.0
	for _, r := range results {
		if math.Abs(r.Z) > math.Abs(bestZ) {
			bestZ = r.Z
			bestSymbol = r.Element
		}
	}
	return bestSymbol, bestZ
}
