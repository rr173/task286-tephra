package composition

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"task286-tephra/internal/model"
)

// ContentHash 计算观测的内容指纹（幂等键）。
//
// 指纹覆盖：样品编号、颗粒数、单位与全部元素含量/误差的有序序列化。
// 相同来源（batch_id + sample_no）且指纹相同的观测视为重复导入，幂等跳过；
// 指纹不同则视为新观测（同一样品号允许存在多个分析回合）。
func ContentHash(batchID, sampleNo, unit string, elements map[string]model.ElementValue) string {
	keys := make([]string, 0, len(elements))
	for k := range elements {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	payload := struct {
		BatchID string                    `json:"batch_id"`
		Sample  string                    `json:"sample_no"`
		Unit    string                    `json:"unit"`
		Elems   map[string]model.ElementValue `json:"elements"`
	}{
		BatchID: batchID,
		Sample:  sampleNo,
		Unit:    unit,
		Elems:   elements,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		// elements 已通过校验，JSON 序列化不会失败；失败时退化为哈希零值。
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// BatchStatistics 描述一批观测的批内统计量（按元素）。
type BatchStatistics struct {
	BatchID   string              `json:"batch_id"`
	Count     int                 `json:"count"`      // 参与统计的观测数
	Means     map[string]float64  `json:"means"`      // 元素均值
	StdDevs   map[string]float64  `json:"std_devs"`   // 样本标准偏差
	StdErrors map[string]float64  `json:"std_errors"` // 均值标准误
	MeanErrs  map[string]float64  `json:"mean_errs"`  // 平均分析误差
	Elements  []string            `json:"elements"`   // 参与统计的元素（目录序）
}

// ComputeStatistics 计算一批观测的批内统计量。
//
// 每个元素取观测中共同出现的元素；观测不足 2 条时标准偏差为 0
// （此时距离项退化为仅由分析误差决定）。任一元素缺失的观测在该元素上
// 视为缺失（不参与均值），但至少需要 5 个共同元素才有统计意义。
func ComputeStatistics(observations []model.Observation) (*BatchStatistics, error) {
	if len(observations) < 2 {
		return nil, fmt.Errorf("%w: at least 2 observations needed for batch statistics", model.ErrInvalidInput)
	}
	stats := &BatchStatistics{
		Count:     len(observations),
		Means:     map[string]float64{},
		StdDevs:   map[string]float64{},
		StdErrors: map[string]float64{},
		MeanErrs:  map[string]float64{},
	}
	// 先收集共同元素
	common := Symbols()
	for _, o := range observations {
		cur := make(map[string]bool, len(o.Elements))
		for k := range o.Elements {
			cur[k] = true
		}
		var next []string
		for _, s := range common {
			if cur[s] {
				next = append(next, s)
			}
		}
		common = next
		if len(common) < 5 {
			return nil, fmt.Errorf("%w: fewer than 5 common elements across observations", model.ErrInvalidInput)
		}
	}
	stats.Elements = common

	// 均值
	sums := make(map[string]float64, len(common))
	sumErrs := make(map[string]float64, len(common))
	counts := make(map[string]int, len(common))
	for _, o := range observations {
		for _, s := range common {
			if ev, ok := o.Elements[s]; ok {
				sums[s] += ev.Value
				sumErrs[s] += ev.Error
				counts[s]++
			}
		}
	}
	for _, s := range common {
		stats.Means[s] = sums[s] / float64(counts[s])
		stats.MeanErrs[s] = sumErrs[s] / float64(counts[s])
	}

	// 样本标准偏差
	sq := make(map[string]float64, len(common))
	for _, o := range observations {
		for _, s := range common {
			if ev, ok := o.Elements[s]; ok {
				d := ev.Value - stats.Means[s]
				sq[s] += d * d
			}
		}
	}
	for _, s := range common {
		n := float64(counts[s])
		if n > 1 {
			stats.StdDevs[s] = math.Sqrt(sq[s] / (n - 1))
		} else {
			stats.StdDevs[s] = 0
		}
		stats.StdErrors[s] = stats.StdDevs[s] / math.Sqrt(n)
	}
	return stats, nil
}
