// Package normalize 实现玻璃成分观测的误差标准化。
//
// 标准化语义：把原始观测（含量 + 1σ 分析误差）转换为"标准化的成分指纹"，
// 使不同实验室、不同分析批次的观测可以在同一误差尺度上比较。
// 标准化后的观测满足：含量在 [0,100] 内、误差为正、总量为 100 wt%。
package normalize

import (
	"fmt"
	"math"

	"task286-tephra/internal/model"
)

// Version 是当前标准化算法的版本标识。相关快照固定发布时刻的版本号，
// 算法升级后旧快照保持其记录值不变。
const Version = "2.1"

// Standardizer 执行观测误差标准化。零值可用。
type Standardizer struct {
	// MinError 是任意元素误差的下限（wt%）。分析误差低于该值时视为不可信，
	// 抬升到该值，防止过小的误差主导指纹距离。
	MinError float64
}

// NewStandardizer 构造默认标准化器。
func NewStandardizer() *Standardizer {
	return &Standardizer{MinError: 0.05}
}

// Standardize 把观测的元素值归一化到 100 wt% 并传播误差，
// 返回标准化后的元素表。归一化失败（如负含量、非正误差）返回领域错误。
func (s *Standardizer) Standardize(elements map[string]model.ElementValue) (map[string]model.ElementValue, error) {
	if len(elements) == 0 {
		return nil, fmt.Errorf("%w: empty element set", model.ErrInvalidInput)
	}
	total := 0.0
	for _, ev := range elements {
		if ev.Value < 0 {
			return nil, fmt.Errorf("%w: negative concentration", model.ErrInvalidInput)
		}
		if ev.Error <= 0 {
			return nil, fmt.Errorf("%w: non-positive error", model.ErrNegativeError)
		}
		total += ev.Value
	}
	if total <= 0 {
		return nil, fmt.Errorf("%w: total must be positive", model.ErrInvalidInput)
	}
	out := make(map[string]model.ElementValue, len(elements))
	for symbol, ev := range elements {
		normValue := ev.Value * 100 / total
		normErr := ev.Error * 100 / total
		if normErr < s.MinError {
			normErr = s.MinError
		}
		if normErr > 5.0 {
			// 单元素误差上限：超过 5 wt% 的误差说明该元素无法可靠定量，
			// 在指纹比较中保留但权重极低（由 distance 包处理）。
			normErr = 5.0
		}
		out[symbol] = model.ElementValue{Value: normValue, Error: normErr}
	}
	return out, nil
}

// StandardizedDifference 计算两个标准化元素值之间的标准化差异（t 分数）。
//
//	z = (a - b) / sqrt(a_err² + b_err²)
//
// |z| > 2 视为该元素上两观测显著不同（约 95% 置信）。
func StandardizedDifference(a, b model.ElementValue) float64 {
	denom := math.Sqrt(a.Error*a.Error + b.Error*b.Error)
	if denom <= 0 {
		return 0
	}
	return (a.Value - b.Value) / denom
}

// Quality 描述一次观测的标准化质量。
type Quality struct {
	Elements       int     // 有效元素数
	AboveDetection float64 // 高于检测限（>2×最小误差）的元素占比
	CV             float64 // 主要元素（SiO2/Al2O3/K2O/FeO）变异系数的均值
	Usable         bool    // 是否可用于指纹比较
	Reason         string  // 不可用原因
}

// AssessQuality 评估标准化观测的质量。
func (s *Standardizer) AssessQuality(elements map[string]model.ElementValue) Quality {
	q := Quality{Elements: len(elements), AboveDetection: 0}
	if len(elements) < 5 {
		q.Usable = false
		q.Reason = "too few elements"
		return q
	}
	detected := 0
	var cvs []float64
	for symbol, ev := range elements {
		if ev.Value > 2*s.MinError {
			detected++
		}
		if ev.Error > 0 {
			cvs = append(cvs, ev.Error/ev.Value)
		}
		_ = symbol
	}
	q.AboveDetection = float64(detected) / float64(len(elements))
	if len(cvs) > 0 {
		sum := 0.0
		for _, c := range cvs {
			sum += c
		}
		q.CV = sum / float64(len(cvs))
	}
	q.Usable = q.AboveDetection >= 0.8
	if !q.Usable {
		q.Reason = "most elements below detection"
	}
	return q
}
