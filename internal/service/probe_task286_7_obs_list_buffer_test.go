package service

import (
	"testing"

	"task286-tephra/internal/model"
	"task286-tephra/internal/store"
)

func newProbeSvc(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st)
}

func probeObs(sampleNo string, k2o float64) model.Observation {
	return model.Observation{
		SampleNo:   sampleNo,
		GrainCount: 8,
		Elements: map[string]model.ElementValue{
			"SiO2": {Value: 72.4, Error: 0.4}, "TiO2": {Value: 0.28, Error: 0.05},
			"Al2O3": {Value: 12.6, Error: 0.2}, "FeO": {Value: 2.9, Error: 0.15},
			"MnO": {Value: 0.08, Error: 0.04}, "MgO": {Value: 0.35, Error: 0.05},
			"CaO": {Value: 1.55, Error: 0.1}, "Na2O": {Value: 4.1, Error: 0.2},
			"K2O": {Value: 3.1 + k2o, Error: 0.15}, "P2O5": {Value: 0.06, Error: 0.03},
		},
	}
}

func seedComparableBatch(t *testing.T, svc *Service, name string, top, bottom float64) *model.Batch {
	t.Helper()
	b, err := svc.CreateBatch(&model.Batch{Name: name, TopDepth: top, BottomDepth: bottom})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	for i, suffix := range []string{"01", "02", "03"} {
		o := probeObs("G-"+suffix, float64(i)*0.05-0.05)
		o.BatchID = b.ID
		if _, _, err := svc.ImportObservation(&o); err != nil {
			t.Fatalf("import: %v", err)
		}
	}
	if _, _, err := svc.StandardizeBatch(b.ID); err != nil {
		t.Fatalf("standardize: %v", err)
	}
	if _, err := svc.TransitionBatch(b.ID, model.BatchReady); err != nil {
		t.Fatalf("ready: %v", err)
	}
	return b
}

func TestListObservationsIndependentSlices(t *testing.T) {
	svc := newProbeSvc(t)
	b := seedComparableBatch(t, svc, "KR-S1", 50.0, 50.8)
	first, err := svc.Store().ListObservationsByBatch(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Store().ListObservationsByBatch(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 || len(second) == 0 {
		t.Fatal("expected observations")
	}
	second[0].SampleNo = "MUTATED"
	if first[0].SampleNo == "MUTATED" {
		t.Fatal("second slice mutation affected first slice backing array")
	}
}
