// Package httpapi 提供 HTTP JSON API 层。
//
// 所有路由以 /api 为前缀；错误统一映射为 JSON：
// {"error": "<message>"}，状态码按领域错误语义映射。
package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"task286-tephra/internal/model"
	"task286-tephra/internal/service"
)

// Server 是 HTTP 处理器集合。
type Server struct {
	svc *service.Service
	mux *http.ServeMux
}

// New 构造 HTTP Server 并注册全部路由。
func New(svc *service.Service) *Server {
	s := &Server{svc: svc, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler 返回可挂载的 http.Handler。
func (s *Server) Handler() http.Handler {
	return s.mux
}

// routes 注册全部路由（≥20 个 API）。
func (s *Server) routes() {
	// 通用
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/selfcheck", s.handleSelfCheck)
	s.mux.HandleFunc("GET /api/elements", s.handleElements)
	s.mux.HandleFunc("GET /api/audits", s.handleAudits)

	// 灰层批次
	s.mux.HandleFunc("POST /api/batches", s.handleCreateBatch)
	s.mux.HandleFunc("GET /api/batches", s.handleListBatches)
	s.mux.HandleFunc("GET /api/batches/{id}", s.handleGetBatch)
	s.mux.HandleFunc("PATCH /api/batches/{id}/status", s.handleTransitionBatch)
	s.mux.HandleFunc("POST /api/batches/{id}/seal", s.handleSealBatch)
	s.mux.HandleFunc("GET /api/batches/{id}/observations", s.handleListObservations)
	s.mux.HandleFunc("GET /api/batches/{id}/correlations", s.handleListBatchCorrelations)
	s.mux.HandleFunc("GET /api/batches/{id}/snapshots", s.handleListBatchSnapshots)
	s.mux.HandleFunc("POST /api/batches/{id}/standardize", s.handleStandardizeBatch)
	s.mux.HandleFunc("POST /api/batches/{id}/screen", s.handleScreenRedeposition)

	// 成分观测
	s.mux.HandleFunc("POST /api/observations", s.handleImportObservation)
	s.mux.HandleFunc("POST /api/observations/batch", s.handleImportObservations)
	s.mux.HandleFunc("GET /api/observations/{id}", s.handleGetObservation)
	s.mux.HandleFunc("POST /api/observations/{id}/standardize", s.handleStandardizeObservation)
	s.mux.HandleFunc("POST /api/observations/{id}/mark-redeposited", s.handleMarkRedeposited)
	s.mux.HandleFunc("POST /api/observations/{id}/exclude", s.handleExcludeObservation)
	s.mux.HandleFunc("POST /api/observations/{id}/restore", s.handleRestoreObservation)

	// 相关关系
	s.mux.HandleFunc("POST /api/correlations", s.handleCreateCorrelation)
	s.mux.HandleFunc("GET /api/correlations", s.handleListCorrelations)
	s.mux.HandleFunc("GET /api/correlations/{id}", s.handleGetCorrelation)
	s.mux.HandleFunc("POST /api/correlations/{id}/recompute", s.handleRecomputeCorrelation)
	s.mux.HandleFunc("POST /api/correlations/{id}/adjudicate", s.handleAdjudicateCorrelation)
	s.mux.HandleFunc("GET /api/correlations/rank", s.handleRankCorrelations)

	// 相关快照
	s.mux.HandleFunc("POST /api/snapshots", s.handleCreateSnapshot)
	s.mux.HandleFunc("GET /api/snapshots", s.handleListSnapshots)
	s.mux.HandleFunc("GET /api/snapshots/{id}", s.handleGetSnapshot)
	s.mux.HandleFunc("POST /api/snapshots/{id}/publish", s.handlePublishSnapshot)
	s.mux.HandleFunc("POST /api/snapshots/{left}/supersede/{right}", s.handleSupersedeSnapshot)
	s.mux.HandleFunc("GET /api/snapshots/{left}/diff/{right}", s.handleDiffSnapshots)
}

// writeJSON 写 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeErr 按领域错误映射 HTTP 状态码。
func writeErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch err {
	case model.ErrNotFound:
		status = http.StatusNotFound
	case model.ErrInvalidInput, model.ErrInvalidState, model.ErrUnitMismatch,
		model.ErrNegativeError, model.ErrInvertedRange, model.ErrCorrNotReady:
		status = http.StatusBadRequest
	case model.ErrConflict:
		status = http.StatusConflict
	case model.ErrSealed:
		status = http.StatusForbidden
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// decodeJSON 解码请求体。
func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: invalid JSON body: %v", model.ErrInvalidInput, err)
	}
	return nil
}

// pathVar 读取路径变量（Go 1.22+ 模式路由）。
func pathVar(r *http.Request, name string) string {
	return strings.TrimSpace(r.PathValue(name))
}
