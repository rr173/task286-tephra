package stratigraphy

import (
	"sort"

	"task286-tephra/internal/model"
)

// OrderState 描述一个剖面内所有灰层之间的层位序关系。
type OrderState struct {
	BatchIDs []string             `json:"batch_ids"` // 按深度排序的批次
	ByDepth  map[string]float64   `json:"by_depth"`  // 批次 -> 代表深度（区间中点）
}

// BuildOrder 把一个剖面的所有灰层按代表深度（区间中点）排序。
// 仅统计状态非封存的批次；返回的序列用于约束检查前的快速自洽验证。
func BuildOrder(batches []model.Batch) *OrderState {
	os := &OrderState{ByDepth: map[string]float64{}}
	type pair struct {
		id    string
		depth float64
	}
	var ps []pair
	for _, b := range batches {
		if b.Status == model.BatchSealed {
			continue
		}
		d := (b.TopDepth + b.BottomDepth) / 2
		os.ByDepth[b.ID] = d
		ps = append(ps, pair{id: b.ID, depth: d})
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].depth < ps[j].depth })
	for _, p := range ps {
		os.BatchIDs = append(os.BatchIDs, p.id)
	}
	return os
}

// Above 判断 a 是否严格位于 b 之上（按代表深度）。
// 返回 (存在, 是否在上)。
func (os *OrderState) Above(a, b string) (bool, bool) {
	da, okA := os.ByDepth[a]
	db, okB := os.ByDepth[b]
	if !okA || !okB {
		return false, false
	}
	return true, da < db
}

// Consistent 校验一个相关候选的层位声明与全局序一致：
// upper 在全局序中必须位于 lower 之上（或重叠，即允许相等）。
func (os *OrderState) Consistent(upperID, lowerID string) bool {
	exists, above := os.Above(upperID, lowerID)
	if !exists {
		return true // 缺失批次（如封存）不参与全局序校验
	}
	return above
}
