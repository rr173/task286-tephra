package fingerprint

import (
	"math"
	"testing"

	"task286-tephra/internal/composition"
	"task286-tephra/internal/model"
)

func statsFrom(elements []map[string]model.ElementValue) *composition.BatchStatistics {
	var obs []model.Observation
	for i, e := range elements {
		obs = append(obs, model.Observation{ID: string(rune('a' + i)), Elements: e})
	}
	st, err := composition.ComputeStatistics(obs)
	if err != nil {
		panic(err)
	}
	return st
}

func baseElements(delta map[string]float64) map[string]model.ElementValue {
	out := map[string]model.ElementValue{
		"SiO2": {Value: 72.4, Error: 0.4}, "TiO2": {Value: 0.28, Error: 0.05},
		"Al2O3": {Value: 12.6, Error: 0.2}, "FeO": {Value: 2.9, Error: 0.15},
		"MnO": {Value: 0.08, Error: 0.04}, "MgO": {Value: 0.35, Error: 0.05},
		"CaO": {Value: 1.55, Error: 0.1}, "Na2O": {Value: 4.1, Error: 0.2},
		"K2O": {Value: 3.1, Error: 0.15}, "P2O5": {Value: 0.06, Error: 0.03},
	}
	for k, d := range delta {
		ev := out[k]
		ev.Value += d
		out[k] = ev
	}
	return out
}

func identicalStats() (*composition.BatchStatistics, *composition.BatchStatistics) {
	return statsFrom([]map[string]model.ElementValue{baseElements(nil), baseElements(nil)}),
		statsFrom([]map[string]model.ElementValue{baseElements(nil), baseElements(nil)})
}

func TestCompareIdentical(t *testing.T) {
	a, b := identicalStats()
	v, err := Compare(a, b)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if v.Distance > 1e-9 {
		t.Errorf("identical batches should have ~0 distance, got %v", v.Distance)
	}
	if !v.Compatible {
		t.Error("identical batches should be compatible")
	}
	if v.PValue < 0.99 {
		t.Errorf("p-value for identical batches should be near 1, got %v", v.PValue)
	}
}

func TestCompareDifferent(t *testing.T) {
	a := statsFrom([]map[string]model.ElementValue{baseElements(nil), baseElements(nil)})
	b := statsFrom([]map[string]model.ElementValue{baseElements(map[string]float64{"SiO2": 4.0, "K2O": 1.5}), baseElements(map[string]float64{"SiO2": 4.0, "K2O": 1.5})})
	v, err := Compare(a, b)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if v.Distance < 10 {
		t.Errorf("clearly different batches should have large distance, got %v", v.Distance)
	}
	if v.Compatible {
		t.Error("clearly different batches should NOT be compatible")
	}
	if v.PValue > 0.05 {
		t.Errorf("p-value should be tiny, got %v", v.PValue)
	}
}

func TestUpperChiSquare(t *testing.T) {
	// χ²(1, x=3.841) ≈ 0.05
	p := UpperChiSquare(3.841, 1)
	if math.Abs(p-0.05) > 0.01 {
		t.Errorf("chi-square(1, 3.841) = %v, want ~0.05", p)
	}
	// χ²(2, x=5.991) ≈ 0.05
	p = UpperChiSquare(5.991, 2)
	if math.Abs(p-0.05) > 0.01 {
		t.Errorf("chi-square(2, 5.991) = %v, want ~0.05", p)
	}
	// 大 x 时 p 趋近 0
	if p := UpperChiSquare(1000, 10); p > 1e-6 {
		t.Errorf("chi-square(10, 1000) = %v, want ~0", p)
	}
	// x<=0 时 p = 1
	if p := UpperChiSquare(0, 5); p != 1 {
		t.Errorf("chi-square(5, 0) = %v, want 1", p)
	}
}

func TestRankDistance(t *testing.T) {
	if RankDistance(0.5, 10) != RankStrong {
		t.Error("ratio<1 should be strong")
	}
	if RankDistance(15, 10) != RankModerate {
		t.Error("ratio<2 should be moderate")
	}
	if RankDistance(30, 10) != RankWeak {
		t.Error("ratio>2 should be weak")
	}
}
