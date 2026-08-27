package httpapi

import (
	"net/http"
)

// createSnapshotRequest 创建快照请求体。
type createSnapshotRequest struct {
	Name string `json:"name"`
	Note string `json:"note"`
}

// handleCreateSnapshot POST /api/snapshots
func (s *Server) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	var req createSnapshotRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	snap, err := s.svc.CreateSnapshot(req.Name, req.Note)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

// handleListSnapshots GET /api/snapshots
func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	snaps, err := s.svc.ListSnapshots()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshots": snaps})
}

// handleGetSnapshot GET /api/snapshots/{id}
func (s *Server) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	snap, err := s.svc.GetSnapshot(pathVar(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// handlePublishSnapshot POST /api/snapshots/{id}/publish
func (s *Server) handlePublishSnapshot(w http.ResponseWriter, r *http.Request) {
	snap, err := s.svc.PublishSnapshot(pathVar(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// handleSupersedeSnapshot POST /api/snapshots/{left}/supersede/{right}
func (s *Server) handleSupersedeSnapshot(w http.ResponseWriter, r *http.Request) {
	oldSnap, err := s.svc.SupersedeSnapshot(pathVar(r, "left"), pathVar(r, "right"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, oldSnap)
}

// handleDiffSnapshots GET /api/snapshots/{left}/diff/{right}
func (s *Server) handleDiffSnapshots(w http.ResponseWriter, r *http.Request) {
	diff, err := s.svc.DiffSnapshots(pathVar(r, "left"), pathVar(r, "right"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"diff": diff})
}
