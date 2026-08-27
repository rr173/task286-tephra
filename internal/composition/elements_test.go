package composition

import (
	"math"
	"testing"

	"task286-tephra/internal/model"
)

func sampleElements() map[string]model.ElementValue {
	return map[string]model.ElementValue{
		"SiO2": {Value: 72.4, Error: 0.4}, "TiO2": {Value: 0.28, Error: 0.05},
		"Al2O3": {Value: 12.6, Error: 0.2}, "FeO": {Value: 2.9, Error: 0.15},
		"MnO": {Value: 0.08, Error: 0.04}, "MgO": {Value: 0.35, Error: 0.05},
		"CaO": {Value: 1.55, Error: 0.1}, "Na2O": {Value: 4.1, Error: 0.2},
		"K2O": {Value: 3.1, Error: 0.15}, "P2O5": {Value: 0.06, Error: 0.03},
	}
}

func TestValidateElements(t *testing.T) {
	// 合法输入
	if err := ValidateElements(sampleElements()); err != nil {
		t.Fatalf("valid elements rejected: %v", err)
	}
	// 未知元素
	bad := sampleElements()
	bad["UO2"] = model.ElementValue{Value: 1, Error: 0.1}
	if err := ValidateElements(bad); err == nil {
		t.Fatal("unknown element should be rejected")
	}
	// 负误差
	bad = sampleElements()
	ev := bad["SiO2"]
	ev.Error = -0.1
	bad["SiO2"] = ev
	if err := ValidateElements(bad); err == nil {
		t.Fatal("negative error should be rejected")
	}
	// 元素不足 5 个
	bad = map[string]model.ElementValue{"SiO2": {Value: 72, Error: 0.5}, "Al2O3": {Value: 12, Error: 0.2}}
	if err := ValidateElements(bad); err == nil {
		t.Fatal("too few elements should be rejected")
	}
}

func TestNormalize(t *testing.T) {
	norm, err := Normalize(sampleElements())
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	total := 0.0
	for _, ev := range norm {
		total += ev.Value
	}
	if math.Abs(total-100) > 1e-9 {
		t.Fatalf("normalized total = %.4f, want 100", total)
	}
	// 误差应随比例缩放
	orig := sampleElements()
	sio2Norm := norm["SiO2"]
	ratio := orig["SiO2"].Value / 97.42 // 原始总量
	if math.Abs(sio2Norm.Error-orig["SiO2"].Error*100/97.42) > 1e-6 {
		t.Errorf("error propagation wrong: %v vs %v", sio2Norm.Error, orig["SiO2"].Error*100/97.42)
	}
	_ = ratio
}

func TestCommonElementsAndSymbols(t *testing.T) {
	a := sampleElements()
	b := map[string]model.ElementValue{
		"SiO2": {Value: 71, Error: 0.4}, "Al2O3": {Value: 13, Error: 0.2},
		"FeO": {Value: 2.5, Error: 0.15}, "CaO": {Value: 2.0, Error: 0.1},
		"K2O": {Value: 3.5, Error: 0.15}, "Na2O": {Value: 4.0, Error: 0.2},
	}
	common := CommonElements(a, b)
	if len(common) != 6 {
		t.Fatalf("common elements = %d, want 6", len(common))
	}
	if !Known("al2o3") || !Known("Al2O3") {
		t.Fatal("Known should be case-insensitive")
	}
	if Known("XYZ") {
		t.Fatal("XYZ should not be known")
	}
}

func TestComputeStatistics(t *testing.T) {
	obs := []model.Observation{
		{ID: "o1", Elements: sampleElements()},
		{ID: "o2", Elements: sampleElements()},
	}
	stats, err := ComputeStatistics(obs)
	if err != nil {
		t.Fatalf("statistics: %v", err)
	}
	if stats.Count != 2 {
		t.Fatalf("count = %d, want 2", stats.Count)
	}
	if math.Abs(stats.Means["SiO2"]-72.4) > 1e-9 {
		t.Errorf("SiO2 mean = %v, want 72.4", stats.Means["SiO2"])
	}
	// 两条完全相同的观测：标准偏差应为 0
	if stats.StdDevs["SiO2"] != 0 {
		t.Errorf("std dev should be 0 for identical observations, got %v", stats.StdDevs["SiO2"])
	}
}
