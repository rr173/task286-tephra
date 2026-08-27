package model

import "testing"

func TestTransitionBatch(t *testing.T) {
	cases := []struct {
		from BatchStatus
		to   BatchStatus
		ok   bool
	}{
		{BatchCollecting, BatchReady, true},
		{BatchCollecting, BatchSealed, true},
		{BatchCollecting, BatchReview, false},
		{BatchReady, BatchReview, true},
		{BatchReady, BatchPublished, true},
		{BatchReady, BatchSealed, true},
		{BatchReview, BatchReady, true},
		{BatchReview, BatchSealed, true},
		{BatchReview, BatchPublished, false},
		{BatchPublished, BatchSealed, true},
		{BatchPublished, BatchReady, false},
		{BatchSealed, BatchReady, false},
		{BatchSealed, BatchSealed, false},
	}
	for _, c := range cases {
		err := TransitionBatch(c.from, c.to)
		if (err == nil) != c.ok {
			t.Errorf("TransitionBatch(%s -> %s) ok=%v, want %v (err=%v)", c.from, c.to, err == nil, c.ok, err)
		}
	}
}

func TestTransitionObservation(t *testing.T) {
	cases := []struct {
		from ObservationStatus
		to   ObservationStatus
		ok   bool
	}{
		{ObsRaw, ObsNormalized, true},
		{ObsRaw, ObsRedeposited, true},
		{ObsRaw, ObsExcluded, true},
		{ObsNormalized, ObsRedeposited, true},
		{ObsNormalized, ObsExcluded, true},
		{ObsNormalized, ObsRaw, false},
		{ObsRedeposited, ObsExcluded, true},
		{ObsRedeposited, ObsNormalized, true},
		{ObsExcluded, ObsNormalized, true},
		{ObsExcluded, ObsExcluded, false},
	}
	for _, c := range cases {
		err := TransitionObservation(c.from, c.to)
		if (err == nil) != c.ok {
			t.Errorf("TransitionObservation(%s -> %s) ok=%v, want %v", c.from, c.to, err == nil, c.ok)
		}
	}
}

func TestTransitionCorrelation(t *testing.T) {
	cases := []struct {
		from CorrelationStatus
		to   CorrelationStatus
		ok   bool
	}{
		{CorrCandidate, CorrCompatible, true},
		{CorrCandidate, CorrStratDiff, true},
		{CorrCompatible, CorrConfirmed, true},
		{CorrCompatible, CorrRejected, true},
		{CorrStratDiff, CorrConfirmed, true},
		{CorrStratDiff, CorrRejected, true},
		{CorrCandidate, CorrConfirmed, false},
		{CorrConfirmed, CorrRejected, false},
		{CorrRejected, CorrCompatible, false},
	}
	for _, c := range cases {
		err := TransitionCorrelation(c.from, c.to)
		if (err == nil) != c.ok {
			t.Errorf("TransitionCorrelation(%s -> %s) ok=%v, want %v", c.from, c.to, err == nil, c.ok)
		}
	}
}

func TestTransitionSnapshot(t *testing.T) {
	if err := TransitionSnapshot(SnapDraft, SnapPublished); err != nil {
		t.Fatalf("draft->published should pass: %v", err)
	}
	if err := TransitionSnapshot(SnapPublished, SnapSuperseded); err != nil {
		t.Fatalf("published->superseded should pass: %v", err)
	}
	if err := TransitionSnapshot(SnapSuperseded, SnapPublished); err == nil {
		t.Fatal("superseded->published should fail")
	}
	if err := TransitionSnapshot(SnapDraft, SnapSuperseded); err == nil {
		t.Fatal("draft->superseded should fail")
	}
}

func TestValidDepthRange(t *testing.T) {
	if !ValidDepthRange(0, 1) {
		t.Error("(0,1) should be valid")
	}
	if ValidDepthRange(1, 1) {
		t.Error("(1,1) should be invalid")
	}
	if ValidDepthRange(-1, 1) {
		t.Error("(-1,1) should be invalid")
	}
}
