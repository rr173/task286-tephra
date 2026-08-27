// Package stratigraphy 实现灰层之间的层位关系与相关可行性约束。
//
// 层位语义：深度从剖面顶部向下递增（top_depth < bottom_depth）。
// 给定上层 upper 与下层 lower（声明的相对位置）：
//   - 区间重叠：upper.bottom >= lower.top → 可行（同一事件沉积）；
//   - 相邻：gap = lower.top - upper.bottom，gap <= MaxGap → 可行；
//   - 相距过远：gap > MaxGap → 层位冲突（不太可能同一喷发事件）；
//   - 顺序倒置：upper.top >= lower.bottom（上层区间整体在下层之下）→ 冲突。
package stratigraphy

import (
	"fmt"

	"task286-tephra/internal/model"
)

// MaxGap 是同一事件两灰层允许的最大层位间隔（深度单位，默认 m）。
const MaxGap = 2.0

// DepthInterval 描述一个深度区间。
type DepthInterval struct {
	Top    float64
	Bottom float64
}

// NewInterval 从灰层构造深度区间，校验区间合法性。
func NewInterval(b *model.Batch) (*DepthInterval, error) {
	if !model.ValidDepthRange(b.TopDepth, b.BottomDepth) {
		return nil, fmt.Errorf("%w: batch %s top=%.3f bottom=%.3f", model.ErrInvertedRange, b.ID, b.TopDepth, b.BottomDepth)
	}
	return &DepthInterval{Top: b.TopDepth, Bottom: b.BottomDepth}, nil
}

// Overlap 返回两个区间的重叠量（无重叠为负值，即间隙的相反数）。
func (iv *DepthInterval) Overlap(o *DepthInterval) float64 {
	return mathMin(iv.Bottom, o.Bottom) - mathMax(iv.Top, o.Top)
}

// Gap 返回两个区间之间的间隙；重叠时为 0。
func (iv *DepthInterval) Gap(o *DepthInterval) float64 {
	g := iv.Overlap(o)
	if g < 0 {
		return -g
	}
	return 0
}

// CheckResult 描述一次层位约束检查的结果。
type CheckResult struct {
	OK       bool    `json:"ok"`        // 层位可行
	Gap      float64 `json:"gap"`       // 层位间隙（重叠为 0）
	Overlap  float64 `json:"overlap"`   // 重叠量（可为负）
	Inverted bool    `json:"inverted"`  // 顺序倒置
	Note     string  `json:"note"`
}

// Check 检查 upper（上层）与 lower（下层）的层位可行性。
func Check(upper, lower *DepthInterval) CheckResult {
	res := CheckResult{Overlap: upper.Overlap(lower)}
	if res.Overlap >= 0 {
		res.OK = true
		res.Gap = 0
		res.Note = "intervals overlap"
		return res
	}
	// 区间分离：检查顺序是否一致（upper 必须整体在 lower 之上）
	if upper.Top >= lower.Bottom {
		res.Inverted = true
		res.Gap = upper.Top - lower.Bottom
		res.OK = false
		res.Note = fmt.Sprintf("inverted order: upper interval lies entirely below lower (gap %.3f)", res.Gap)
		return res
	}
	res.Gap = -res.Overlap
	if res.Gap <= MaxGap {
		res.OK = true
		res.Note = fmt.Sprintf("adjacent intervals within gap %.3f", res.Gap)
		return res
	}
	res.OK = false
	res.Note = fmt.Sprintf("stratigraphic gap %.3f exceeds max %.3f", res.Gap, MaxGap)
	return res
}

// CheckBatches 直接基于两个灰层做层位检查；任何区间非法即返回错误。
func CheckBatches(upper, lower *model.Batch) (CheckResult, error) {
	up, err := NewInterval(upper)
	if err != nil {
		return CheckResult{}, err
	}
	lo, err := NewInterval(lower)
	if err != nil {
		return CheckResult{}, err
	}
	return Check(up, lo), nil
}

// Consistence 判断两个灰层是否处于同一剖面且层位顺序自洽
// （用于快速筛选：若既无重叠又存在倒置则必然不相关）。
func Consistence(upper, lower *model.Batch) bool {
	res, err := CheckBatches(upper, lower)
	if err != nil {
		return false
	}
	return res.OK || !res.Inverted
}

func mathMin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func mathMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
