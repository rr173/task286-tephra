package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"task286-tephra/internal/model"
	"task286-tephra/internal/service"
	"task286-tephra/internal/store"
)

func newProbeHTTP(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/probe-http.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(service.New(st)).Handler()
}

func TestInsufficientObservationsReadyMapsBadRequest(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/sealed.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := service.New(st)
	b, err := svc.CreateBatch(&model.Batch{Name: "THIN-1", TopDepth: 1, BottomDepth: 2})
	if err != nil {
		t.Fatal(err)
	}
	o := probeObs("only-one", 0)
	o.BatchID = b.ID
	if _, _, err := svc.ImportObservation(&o); err != nil {
		t.Fatal(err)
	}
	h := New(svc).Handler()
	req := httptest.NewRequest(http.MethodPatch, "/api/batches/"+b.ID+"/status", bytes.NewReader([]byte(`{"status":"ready"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
}

func probeObs(sampleNo string, k2o float64) model.Observation {
	return model.Observation{
		SampleNo: sampleNo, GrainCount: 8,
		Elements: map[string]model.ElementValue{
			"SiO2": {Value: 72.4, Error: 0.4}, "TiO2": {Value: 0.28, Error: 0.05},
			"Al2O3": {Value: 12.6, Error: 0.2}, "FeO": {Value: 2.9, Error: 0.15},
			"MnO": {Value: 0.08, Error: 0.04}, "MgO": {Value: 0.35, Error: 0.05},
			"CaO": {Value: 1.55, Error: 0.1}, "Na2O": {Value: 4.1, Error: 0.2},
			"K2O": {Value: 3.1 + k2o, Error: 0.15}, "P2O5": {Value: 0.06, Error: 0.03},
		},
	}
}
