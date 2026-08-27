package httpapi

import (
	"net/http"

	"task286-tephra/internal/model"
)

// importObservationRequest 导入观测请求体。
type importObservationRequest struct {
	BatchID    string                          `json:"batch_id"`
	SampleNo   string                          `json:"sample_no"`
	GrainCount int                             `json:"grain_count"`
	Elements   map[string]model.ElementValue   `json:"elements"`
	Unit       string                          `json:"unit"`
	Note       string                          `json:"note"`
}

// handleImportObservation POST /api/observations
func (s *Server) handleImportObservation(w http.ResponseWriter, r *http.Request) {
	var req importObservationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	o := &model.Observation{
		BatchID:    req.BatchID,
		SampleNo:   req.SampleNo,
		GrainCount: req.GrainCount,
		Elements:   req.Elements,
		Unit:       req.Unit,
		Note:       req.Note,
	}
	created, skipped, err := s.svc.ImportObservation(o)
	if err != nil {
		writeErr(w, err)
		return
	}
	status := http.StatusCreated
	message := "created"
	if skipped {
		status = http.StatusOK
		message = "duplicate skipped"
	}
	writeJSON(w, status, map[string]any{"observation": created, "skipped": skipped, "message": message})
}

// handleImportObservations POST /api/observations/batch
func (s *Server) handleImportObservations(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Observations []importObservationRequest `json:"observations"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	obs := make([]model.Observation, 0, len(req.Observations))
	for _, ir := range req.Observations {
		obs = append(obs, model.Observation{
			BatchID:    ir.BatchID,
			SampleNo:   ir.SampleNo,
			GrainCount: ir.GrainCount,
			Elements:   ir.Elements,
			Unit:       ir.Unit,
			Note:       ir.Note,
		})
	}
	results := s.svc.ImportObservations(obs)
	writeJSON(w, http.StatusOK, map[string]any{
		"total":   len(results),
		"results": results,
	})
}

// handleGetObservation GET /api/observations/{id}
func (s *Server) handleGetObservation(w http.ResponseWriter, r *http.Request) {
	o, err := s.svc.Store().GetObservation(pathVar(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// handleListObservations GET /api/batches/{id}/observations
func (s *Server) handleListObservations(w http.ResponseWriter, r *http.Request) {
	obs, err := s.svc.Store().ListObservationsByBatch(pathVar(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"observations": obs})
}

// handleStandardizeObservation POST /api/observations/{id}/standardize
func (s *Server) handleStandardizeObservation(w http.ResponseWriter, r *http.Request) {
	o, err := s.svc.StandardizeObservation(pathVar(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// handleStandardizeBatch POST /api/batches/{id}/standardize
func (s *Server) handleStandardizeBatch(w http.ResponseWriter, r *http.Request) {
	ok, warns, err := s.svc.StandardizeBatch(pathVar(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"standardized": ok, "warnings": warns})
}

// handleScreenRedeposition POST /api/batches/{id}/screen
func (s *Server) handleScreenRedeposition(w http.ResponseWriter, r *http.Request) {
	res, err := s.svc.ScreenRedeposition(pathVar(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// statusNoteRequest 状态变更请求体（附注）。
type statusNoteRequest struct {
	Note string `json:"note"`
}

// handleMarkRedeposited POST /api/observations/{id}/mark-redeposited
func (s *Server) handleMarkRedeposited(w http.ResponseWriter, r *http.Request) {
	var req statusNoteRequest
	_ = decodeJSON(r, &req)
	o, err := s.svc.MarkRedeposited(pathVar(r, "id"), req.Note)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// handleExcludeObservation POST /api/observations/{id}/exclude
func (s *Server) handleExcludeObservation(w http.ResponseWriter, r *http.Request) {
	var req statusNoteRequest
	_ = decodeJSON(r, &req)
	o, err := s.svc.ExcludeObservation(pathVar(r, "id"), req.Note)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// handleRestoreObservation POST /api/observations/{id}/restore
func (s *Server) handleRestoreObservation(w http.ResponseWriter, r *http.Request) {
	var req statusNoteRequest
	_ = decodeJSON(r, &req)
	o, err := s.svc.RestoreObservation(pathVar(r, "id"), req.Note)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}
