// Package fingerprint 实现灰层玻璃成分指纹的距离度量与同源显著性检验。
//
// 核心算法：
//  1. 对两个灰层的批内统计量（均值/标准误/平均分析误差），按共同元素构造
//     合并方差，计算标准化距离 D²（加权欧氏距离）；
//  2. D² 在零假设（两灰层同源）下服从 χ² 分布，自由度为共同元素数；
//  3. p 值低于阈值（默认 0.05）则判定"成分显著不同"，不兼容同源；
//     p 值高于阈值则无法拒绝同源，视为"成分相符"。
package fingerprint

import (
	"math"

	"task286-tephra/internal/composition"
	"task286-tephra/internal/model"
)

// Verdict 描述一次指纹比较的结果。
type Verdict struct {
	Distance   float64 `json:"distance"`    // 标准化距离 D²
	DOF        int     `json:"dof"`         // 自由度（共同元素数）
	PValue     float64 `json:"p_value"`     // 卡方上尾概率
	Compatible bool    `json:"compatible"`  // 成分相符（p >= alpha）
	Alpha      float64 `json:"alpha"`       // 显著性阈值
	Elements   []string `json:"elements"`   // 参与比较的元素
	PerElement []ElementContribution `json:"per_element"` // 逐元素贡献
}

// ElementContribution 描述单个元素对距离的贡献。
type ElementContribution struct {
	Element   string  `json:"element"`
	Diff      float64 `json:"diff"`   // 均值差
	Variance  float64 `json:"variance"` // 合并方差
	ChiSquare float64 `json:"chi_square"` // (diff)²/variance
}

// Compare 比较两个灰层的批内统计量。
// 要求两批次在至少 5 个共同元素上都有可用的统计量。
func Compare(a, b *composition.BatchStatistics) (*Verdict, error) {
	if a == nil || b == nil || a.Count < 2 || b.Count < 2 {
		return nil, model.ErrCorrNotReady
	}
	var elements []string
	for _, s := range a.Elements {
		hasB := false
		for _, t := range b.Elements {
			if t == s {
				hasB = true
				break
			}
		}
		if hasB {
			elements = append(elements, s)
		}
	}
	if len(elements) < 5 {
		return nil, model.ErrCorrNotReady
	}

	v := &Verdict{DOF: len(elements), Alpha: 0.05, Elements: elements}
	chi := 0.0
	for _, s := range elements {
		diff := a.Means[s] - b.Means[s]
		// 合并方差：批内均值标准误的平方和 + 分析误差的平方和
		varA := a.StdErrors[s]*a.StdErrors[s] + a.MeanErrs[s]*a.MeanErrs[s]
		varB := b.StdErrors[s]*b.StdErrors[s] + b.MeanErrs[s]*b.MeanErrs[s]
		variance := varA + varB
		if variance <= 0 {
			variance = 1e-6 // 数值保护：方差退化时给极小值
		}
		c := (diff * diff) / variance
		chi += c
		v.PerElement = append(v.PerElement, ElementContribution{
			Element:   s,
			Diff:      diff,
			Variance:  variance,
			ChiSquare: c,
		})
	}
	v.Distance = chi
	v.PValue = UpperChiSquare(chi, float64(v.DOF))
	v.Compatible = v.PValue >= v.Alpha
	return v, nil
}

// UpperChiSquare 计算卡方分布（自由度 df）的上尾概率 P(χ² > x)。
//
// 实现：x <= 0 时返回 1；x > 20·df 时按正态近似（Wilson–Hilferty）；
// 其余情况用 Lanczos 近似的不完全伽马函数（上尾）。
func UpperChiSquare(x, df float64) float64 {
	if x <= 0 {
		return 1.0
	}
	if df <= 0 {
		return 0.0
	}
	if x > 200*df {
		return 0.0
	}
	// Wilson–Hilferty 正态近似：z = ((x/df)^(1/3) - (1 - 2/(9·df))) / sqrt(2/(9·df))
	df3 := 9 * df
	z := (math.Cbrt(x/df) - (1 - 2/df3)) / math.Sqrt(2/df3)
	return 0.5 * erfc(z / math.Sqrt(2))
}

// erfc 互补误差函数（Numerical Recipes 的 erfc 实现）。
func erfc(x float64) float64 {
	z := math.Abs(x)
	t := 1 / (1 + 0.5*z)
	ans := t * math.Exp(-z*z-1.26551223+
		t*(1.00002368+
			t*(0.37409196+
				t*(0.09678418+
					t*(-0.18628806+
						t*(0.27886807+
							t*(-1.13520398+
								t*(1.48851587+
									t*(-0.82215223+
										t*0.17087277)))))))))
	if x >= 0 {
		return ans
	}
	return 2 - ans
}
