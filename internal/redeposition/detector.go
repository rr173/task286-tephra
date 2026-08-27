// Package redeposition 检测灰层玻璃成分观测中的再搬运（reworking）迹象。
//
// 再搬运指玻璃颗粒在沉积后被水流/风搬运到新位置，混入异源颗粒。
// 检测策略：
//  1. 批内离群：单条观测的标准化残差超过 χ² 临界值（p<0.01）即视为离群；
//  2. 成分混合：批内主要元素的变异系数 CV 异常升高（>0.15）提示颗粒混合；
//  3. 系统漂移：按颗粒数排序的累计均值漂移检验（前后半段均值差显著）。
package redeposition

import (
	"math"

	"task286-tephra/internal/composition"
	"task286-tephra/internal/fingerprint"
	"task286-tephra/internal/model"
)

// Result 描述一次再搬运筛查的结果。
type Result struct {
	BatchID      string          `json:"batch_id"`
	Outliers     []Outlier       `json:"outliers"`     // 离群观测
	Mixed        bool            `json:"mixed"`        // 批内成分混合
	CV           float64         `json:"cv"`           // 主要元素平均 CV
	Drift        bool            `json:"drift"`        // 累积漂移
	Conclusion   string          `json:"conclusion"`   // 结论建议
}

// Outlier 描述一条被标记为再搬运候选的观测。
type Outlier struct {
	ObservationID string  `json:"observation_id"`
	SampleNo      string  `json:"sample_no"`
	ChiSquare     float64 `json:"chi_square"` // 标准化残差
	PValue        float64 `json:"p_value"`
	DeviantElement string `json:"deviant_element"` // 贡献最大的元素
	DeviantZ      float64 `json:"deviant_z"`
}

// CVThreshold 是批内成分混合判定的变异系数阈值。
const CVThreshold = 0.15

// Detect 对一批观测执行再搬运筛查。
// observations 必须是已标准化的观测（含 Elements 字段）。
func Detect(batchID string, observations []model.Observation) Result {
	res := Result{BatchID: batchID}
	if len(observations) < 3 {
		res.Conclusion = "insufficient observations for reworking screen"
		return res
	}
	stats, err := composition.ComputeStatistics(observations)
	if err != nil {
		res.Conclusion = "statistics unavailable: " + err.Error()
		return res
	}
	// 主要元素平均 CV
	var cvs []float64
	for _, s := range []string{"SiO2", "Al2O3", "FeO", "K2O"} {
		if sd, ok := stats.StdDevs[s]; ok {
			mean := stats.Means[s]
			if mean > 0 {
				cvs = append(cvs, sd/mean)
			}
		}
	}
	if len(cvs) > 0 {
		sum := 0.0
		for _, c := range cvs {
			sum += c
		}
		res.CV = sum / float64(len(cvs))
	}
	res.Mixed = res.CV > CVThreshold

	// 离群检测：标准化残差
	zs := make(map[string]float64, len(observations))
	for _, o := range observations {
		chi := 0.0
		bestElem := ""
		bestZ := 0.0
		for _, s := range stats.Elements {
			ev, ok := o.Elements[s]
			if !ok {
				continue
			}
			diff := ev.Value - stats.Means[s]
			variance := stats.StdDevs[s]*stats.StdDevs[s] + ev.Error*ev.Error
			if variance <= 0 {
				continue
			}
			c := (diff * diff) / variance
			chi += c
			z := math.Abs(diff) / math.Sqrt(variance)
			if z > bestZ {
				bestZ = z
				bestElem = s
			}
		}
		p := fingerprint.UpperChiSquare(chi, float64(len(stats.Elements)))
		if p < 0.01 {
			res.Outliers = append(res.Outliers, Outlier{
				ObservationID:  o.ID,
				SampleNo:       o.SampleNo,
				ChiSquare:      chi,
				PValue:         p,
				DeviantElement: bestElem,
				DeviantZ:       bestZ,
			})
		}
		zs[o.ID] = bestZ
	}

	// 累积漂移：按观测顺序前后半段主要元素均值差
	res.Drift = detectDrift(observations, stats)
	switch {
	case len(res.Outliers) > 0 && res.Mixed:
		res.Conclusion = "reworking suspected: outliers and mixed composition"
	case len(res.Outliers) > 0:
		res.Conclusion = "reworking suspected: outlier grains"
	case res.Mixed:
		res.Conclusion = "reworking suspected: mixed composition"
	case res.Drift:
		res.Conclusion = "reworking suspected: cumulative drift"
	default:
		res.Conclusion = "no reworking signature detected"
	}
	return res
}

// detectDrift 按观测创建顺序将样本分为前后两半，比较 SiO2 均值。
func detectDrift(observations []model.Observation, stats *composition.BatchStatistics) bool {
	if len(observations) < 6 {
		return false
	}
	half := len(observations) / 2
	sumA, sumB := 0.0, 0.0
	nA, nB := 0, 0
	for i, o := range observations {
		ev, ok := o.Elements["SiO2"]
		if !ok {
			continue
		}
		if i < half {
			sumA += ev.Value
			nA++
		} else {
			sumB += ev.Value
			nB++
		}
	}
	if nA < 2 || nB < 2 {
		return false
	}
	meanA, meanB := sumA/float64(nA), sumB/float64(nB)
	// 用合并标准误判断漂移显著（|diff| > 2×se）
	se := math.Sqrt(2) * (stats.StdDevs["SiO2"] / math.Sqrt(float64(nA)))
	if se <= 0 {
		return false
	}
	return math.Abs(meanA-meanB)/se > 2
}
