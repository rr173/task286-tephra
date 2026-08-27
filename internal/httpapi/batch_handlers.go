package httpapi

import (
	"net/http"

	"task286-tephra/internal/model"
)

// createBatchRequest 创建批次请求体。
type createBatchRequest struct {
	Name        string  `json:"name"`
	Site        string  `json:"site"`
	Formation   string  `json:"formation"`
	TopDepth    float64 `json:"top_depth"`
	BottomDepth float64 `json:"bottom_depth"`
	DepthUnit   string  `json:"depth_unit"`
	Description string  `json:"description"`
}

// handleCreateBatch POST /api/batches
func (s *Server) handleCreateBatch(w http.ResponseWriter, r *http.Request) {
	var req createBatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	b := &model.Batch{
		Name:        req.Name,
		Site:        req.Site,
		Formation:   req.Formation,
		TopDepth:    req.TopDepth,
		BottomDepth: req.BottomDepth,
		DepthUnit:   req.DepthUnit,
		Description: req.Description,
	}
	created, err := s.svc.CreateBatch(b)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// handleListBatches GET /api/batches
func (s *Server) handleListBatches(w http.ResponseWriter, r *http.Request) {
	batches, err := s.svc.ListBatches()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"batches": batches})
}

// handleGetBatch GET /api/batches/{id}
func (s *Server) handleGetBatch(w http.ResponseWriter, r *http.Request) {
	b, err := s.svc.GetBatch(pathVar(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// transitionBatchRequest 状态流转请求体。
type transitionBatchRequest struct {
	Status string `json:"status"`
}

// handleTransitionBatch PATCH /api/batches/{id}/status
func (s *Server) handleTransitionBatch(w http.ResponseWriter, r *http.Request) {
	var req transitionBatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	b, err := s.svc.TransitionBatch(pathVar(r, "id"), model.BatchStatus(req.Status))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// handleSealBatch POST /api/batches/{id}/seal
func (s *Server) handleSealBatch(w http.ResponseWriter, r *http.Request) {
	b, err := s.svc.SealBatch(pathVar(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// handleListBatchCorrelations GET /api/batches/{id}/correlations
func (s *Server) handleListBatchCorrelations(w http.ResponseWriter, r *http.Request) {
	corrs, err := s.svc.ListCorrelationsByBatch(pathVar(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"correlations": corrs})
}

// handleListBatchSnapshots GET /api/batches/{id}/snapshots
// 返回纳入该批次（作为上/下层）的快照列表。
func (s *Server) handleListBatchSnapshots(w http.ResponseWriter, r *http.Request) {
	batchID := pathVar(r, "id")
	if _, err := s.svc.GetBatch(batchID); err != nil {
		writeErr(w, err)
		return
	}
	snaps, err := s.svc.ListSnapshots()
	if err != nil {
		writeErr(w, err)
		return
	}
	var out []model.Snapshot
	for _, sp := range snaps {
		full, err := s.svc.GetSnapshot(sp.ID)
		if err != nil {
			continue
		}
		for _, link := range full.Links {
			if link.UpperBatchID == batchID || link.LowerBatchID == batchID {
				out = append(out, *full)
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshots": out})
}
