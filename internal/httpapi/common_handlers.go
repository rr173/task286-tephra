package httpapi

import (
	"net/http"

	"task286-tephra/internal/composition"
)

// handleHealth 健康检查。
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "task286-tephra",
	})
}

// handleSelfCheck 一致性自检。
func (s *Server) handleSelfCheck(w http.ResponseWriter, r *http.Request) {
	issues, err := s.svc.SelfCheck()
	if err != nil {
		writeErr(w, err)
		return
	}
	if len(issues) > 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "issues": issues})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "issues": []string{}})
}

// handleElements 元素目录。
func (s *Server) handleElements(w http.ResponseWriter, r *http.Request) {
	type elemDTO struct {
		Symbol  string  `json:"symbol"`
		Name    string  `json:"name"`
		Min     float64 `json:"min_value"`
		Max     float64 `json:"max_value"`
		Typical float64 `json:"typical"`
	}
	out := make([]elemDTO, 0, len(composition.Catalog))
	for _, spec := range composition.Catalog {
		out = append(out, elemDTO{Symbol: spec.Symbol, Name: spec.Name, Min: spec.MinValue, Max: spec.MaxValue, Typical: spec.Typical})
	}
	writeJSON(w, http.StatusOK, map[string]any{"elements": out, "unit": "wt%"})
}

// handleAudits 最近审计日志。
func (s *Server) handleAudits(w http.ResponseWriter, r *http.Request) {
	entries, err := s.svc.Store().RecentAudits(100)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audits": entries})
}
