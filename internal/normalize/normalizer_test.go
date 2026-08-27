package normalize

import (
	"math"
	"testing"

	"task286-tephra/internal/model"
)

func sample() map[string]model.ElementValue {
	return map[string]model.ElementValue{
		"SiO2": {Value: 72.4, Error: 0.4}, "TiO2": {Value: 0.28, Error: 0.05},
		"Al2O3": {Value: 12.6, Error: 0.2}, "FeO": {Value: 2.9, Error: 0.15},
		"MnO": {Value: 0.08, Error: 0.04}, "MgO": {Value: 0.35, Error: 0.05},
		"CaO": {Value: 1.55, Error: 0.1}, "Na2O": {Value: 4.1, Error: 0.2},
		"K2O": {Value: 3.1, Error: 0.15}, "P2O5": {Value: 0.06, Error: 0.03},
	}
}

func TestStandardizeTotal(t *testing.T) {
	s := NewStandardizer()
	out, err := s.Standardize(sample())
	if err != nil {
		t.Fatalf("standardize: %v", err)
	}
	total := 0.0
	for _, ev := range out {
		total += ev.Value
	}
	if math.Abs(total-100) > 1e-6 {
		t.Errorf("total = %v, want 100", total)
	}
}

func TestStandardizeErrors(t *testing.T) {
	s := NewStandardizer()
	// 空集合
	if _, err := s.Standardize(map[string]model.ElementValue{}); err == nil {
		t.Error("empty set should fail")
	}
	// 负含量
	bad := sample()
	ev := bad["SiO2"]
	ev.Value = -1
	bad["SiO2"] = ev
	if _, err := s.Standardize(bad); err == nil {
		t.Error("negative concentration should fail")
	}
	// 非正误差
	bad = sample()
	ev = bad["FeO"]
	ev.Error = 0
	bad["FeO"] = ev
	if _, err := s.Standardize(bad); err == nil {
		t.Error("zero error should fail")
	}
}

func TestStandardizedDifference(t *testing.T) {
	a := model.ElementValue{Value: 72.0, Error: 0.4}
	b := model.ElementValue{Value: 72.0, Error: 0.4}
	if z := StandardizedDifference(a, b); z != 0 {
		t.Errorf("identical values should have z=0, got %v", z)
	}
	c := model.ElementValue{Value: 74.0, Error: 0.4}
	z := StandardizedDifference(a, c)
	if math.Abs(z) < 2 {
		t.Errorf("2wt%% difference with 0.4 error should be significant, got %v", z)
	}
}

func TestAssessQuality(t *testing.T) {
	s := NewStandardizer()
	q := s.AssessQuality(sample())
	if !q.Usable {
		t.Errorf("good sample should be usable: %+v", q)
	}
	if q.AboveDetection < 0.8 {
		t.Errorf("above detection ratio should be high: %+v", q)
	}
	// 元素不足
	weak := map[string]model.ElementValue{"SiO2": {Value: 72, Error: 0.5}}
	q = s.AssessQuality(weak)
	if q.Usable {
		t.Error("too few elements should not be usable")
	}
}

func TestCompareObservations(t *testing.T) {
	a := sample()
	b := sample()
	ev := b["K2O"]
	ev.Value += 1.0
	b["K2O"] = ev
	results := CompareObservations(a, b)
	found := false
	for _, r := range results {
		if r.Element == "K2O" && r.Significant {
			found = true
		}
	}
	if !found {
		t.Error("K2O difference should be flagged significant")
	}
	sym, z := MaxAbsZ(results)
	if sym != "K2O" {
		t.Errorf("max abs z should be K2O, got %s", sym)
	}
	if math.Abs(z) < 2 {
		t.Errorf("max z should exceed 2, got %v", z)
	}
}
