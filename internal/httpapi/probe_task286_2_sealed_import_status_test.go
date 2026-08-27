package httpapi

import (
	"bytes"
	"encoding/json"
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

func TestSealedBatchImportMapsForbidden(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/sealed.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := service.New(st)
	b, err := svc.CreateBatch(&model.Batch{Name: "SEAL-1", TopDepth: 1, BottomDepth: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SealBatch(b.ID); err != nil {
		t.Fatal(err)
	}
	h := New(svc).Handler()
	body, _ := json.Marshal(map[string]any{
		"batch_id": b.ID, "sample_no": "X-1", "grain_count": 1,
		"elements": map[string]any{"SiO2": map[string]any{"value": 70.0, "error": 0.5}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/observations", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=403 body=%s", rec.Code, rec.Body.String())
	}
}
