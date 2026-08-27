package fingerprint

import (
	"math"
)

// Significance 描述显著性检验的补充指标（除 χ² p 值之外的稳健性度量）。
type Significance struct {
	// MeanAbsZ 是所有共同元素 |z| 的均值；>1 提示系统性偏移。
	MeanAbsZ float64 `json:"mean_abs_z"`
	// MaxAbsZ 是最大 |z| 及其元素。
	MaxAbsZ   float64 `json:"max_abs_z"`
	MaxZElem  string  `json:"max_z_elem"`
	// Overlaps 是 |z| <= 2 的元素占比（经验相容比例）。
	Overlaps float64 `json:"overlaps"`
}

// RobustSignificance 基于逐元素标准化差异计算稳健性指标。
// 输入为 compare 阶段的 z 值序列（a 批均值对 b 批均值，按合并方差标准化）。
func RobustSignificance(zs map[string]float64) Significance {
	sig := Significance{}
	if len(zs) == 0 {
		return sig
	}
	sum := 0.0
	maxZ := 0.0
	overlap := 0
	for elem, z := range zs {
		az := math.Abs(z)
		sum += az
		if az > maxZ {
			maxZ = az
			sig.MaxZElem = elem
		}
		if az <= 2 {
			overlap++
		}
	}
	sig.MeanAbsZ = sum / float64(len(zs))
	sig.MaxAbsZ = maxZ
	sig.Overlaps = float64(overlap) / float64(len(zs))
	return sig
}

// DistanceRank 描述距离的相对强度分级（用于裁决建议）。
type DistanceRank string

const (
	RankStrong  DistanceRank = "strong"  // 距离小，成分高度一致
	RankModerate DistanceRank = "moderate" // 距离中等
	RankWeak    DistanceRank = "weak"    // 距离大，成分差异明显
)

// RankDistance 根据 D² 与自由度的比值给距离分级。
// D²/df < 1 为 strong；< 2 为 moderate；否则 weak。
func RankDistance(distance float64, dof int) DistanceRank {
	if dof <= 0 {
		return RankWeak
	}
	ratio := distance / float64(dof)
	switch {
	case ratio < 1:
		return RankStrong
	case ratio < 2:
		return RankModerate
	default:
		return RankWeak
	}
}

// SuggestAdjudication 根据指纹与层位结果给出裁决建议。
//   - 成分相符且层位可行 → 建议确认（可相关）；
//   - 成分相符但层位冲突 → 建议否决（再搬运或层位矛盾）；
//   - 成分不符 → 建议否决（不同喷发来源）。
func SuggestAdjudication(compatible, stratOK bool) string {
	switch {
	case compatible && stratOK:
		return "confirm"
	case !compatible:
		return "reject"
	default:
		return "reject"
	}
}
