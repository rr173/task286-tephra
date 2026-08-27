package httpapi

import (
	"net/http"
)

// createCorrelationRequest 创建相关候选请求体。
type createCorrelationRequest struct {
	UpperBatchID string `json:"upper_batch_id"`
	LowerBatchID string `json:"lower_batch_id"`
}

// handleCreateCorrelation POST /api/correlations
func (s *Server) handleCreateCorrelation(w http.ResponseWriter, r *http.Request) {
	var req createCorrelationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	c, err := s.svc.CreateCorrelation(req.UpperBatchID, req.LowerBatchID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// handleListCorrelations GET /api/correlations
func (s *Server) handleListCorrelations(w http.ResponseWriter, r *http.Request) {
	corrs, err := s.svc.ListCorrelations()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"correlations": corrs})
}

// handleGetCorrelation GET /api/correlations/{id}
func (s *Server) handleGetCorrelation(w http.ResponseWriter, r *http.Request) {
	c, err := s.svc.GetCorrelation(pathVar(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// handleRecomputeCorrelation POST /api/correlations/{id}/recompute
func (s *Server) handleRecomputeCorrelation(w http.ResponseWriter, r *http.Request) {
	c, err := s.svc.RecomputeCorrelation(pathVar(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// adjudicateRequest 裁决请求体。
type adjudicateRequest struct {
	Verdict string `json:"verdict"` // confirmed | rejected
	Actor   string `json:"actor"`
	Note    string `json:"note"`
}

// handleAdjudicateCorrelation POST /api/correlations/{id}/adjudicate
func (s *Server) handleAdjudicateCorrelation(w http.ResponseWriter, r *http.Request) {
	var req adjudicateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	c, err := s.svc.AdjudicateCorrelation(pathVar(r, "id"), req.Verdict, req.Actor, req.Note)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// handleRankCorrelations GET /api/correlations/rank
func (s *Server) handleRankCorrelations(w http.ResponseWriter, r *http.Request) {
	scores, err := s.svc.RankCorrelations()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ranked": scores})
}
