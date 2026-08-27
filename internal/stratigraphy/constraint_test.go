package stratigraphy

import (
	"math"
	"testing"

	"task286-tephra/internal/model"
)

func TestCheckOverlap(t *testing.T) {
	upper := &DepthInterval{Top: 10.0, Bottom: 10.8}
	lower := &DepthInterval{Top: 11.0, Bottom: 11.9}
	res := Check(upper, lower)
	if !res.OK {
		t.Fatalf("adjacent intervals within gap should be OK: %+v", res)
	}
	if math.Abs(res.Gap-0.2) > 1e-9 {
		t.Errorf("gap = %v, want 0.2", res.Gap)
	}
}

func TestCheckActualOverlap(t *testing.T) {
	upper := &DepthInterval{Top: 10.0, Bottom: 11.2}
	lower := &DepthInterval{Top: 11.0, Bottom: 12.0}
	res := Check(upper, lower)
	if !res.OK {
		t.Fatalf("overlapping intervals should be OK: %+v", res)
	}
	if res.Gap != 0 {
		t.Errorf("overlap gap should be 0, got %v", res.Gap)
	}
	if res.Overlap <= 0 {
		t.Errorf("overlap should be positive, got %v", res.Overlap)
	}
}

func TestCheckFarApart(t *testing.T) {
	upper := &DepthInterval{Top: 10.0, Bottom: 10.8}
	lower := &DepthInterval{Top: 80.0, Bottom: 80.7}
	res := Check(upper, lower)
	if res.OK {
		t.Fatal("far apart intervals should be conflicting")
	}
	if res.Inverted {
		t.Error("this is not inverted, it is just far apart")
	}
	if math.Abs(res.Gap-69.2) > 1e-9 {
		t.Errorf("gap = %v, want 69.2", res.Gap)
	}
}

func TestCheckInverted(t *testing.T) {
	upper := &DepthInterval{Top: 80.0, Bottom: 80.7}
	lower := &DepthInterval{Top: 10.0, Bottom: 10.8}
	res := Check(upper, lower)
	if res.OK {
		t.Fatal("inverted order should be conflicting")
	}
	if !res.Inverted {
		t.Error("should be flagged as inverted")
	}
}

func TestCheckBatchesAndOrder(t *testing.T) {
	upper := &model.Batch{ID: "u", TopDepth: 10.0, BottomDepth: 10.8}
	lower := &model.Batch{ID: "l", TopDepth: 11.0, BottomDepth: 11.9}
	res, err := CheckBatches(upper, lower)
	if err != nil {
		t.Fatalf("check batches: %v", err)
	}
	if !res.OK {
		t.Fatal("should be OK")
	}
	// 全局序
	os := BuildOrder([]model.Batch{*upper, *lower})
	exists, above := os.Above("u", "l")
	if !exists || !above {
		t.Errorf("upper should be above lower in global order: exists=%v above=%v", exists, above)
	}
	exists, above = os.Above("l", "u")
	if !exists || above {
		t.Errorf("lower should NOT be above upper: exists=%v above=%v", exists, above)
	}
}
