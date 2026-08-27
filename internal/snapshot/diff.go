package snapshot

import (
	"sort"

	"task286-tephra/internal/model"
)

// DiffEntry 描述两个快照之间一条关系的差异。
type DiffEntry struct {
	CorrelationID string  `json:"correlation_id"`
	UpperBatchID  string  `json:"upper_batch_id"`
	LowerBatchID  string  `json:"lower_batch_id"`
	LeftStatus    string  `json:"left_status,omitempty"`
	RightStatus   string  `json:"right_status,omitempty"`
	LeftDistance  *float64 `json:"left_distance,omitempty"`
	RightDistance *float64 `json:"right_distance,omitempty"`
	Change        string  `json:"change"` // added / removed / status_changed / distance_changed / unchanged
}

// Diff 比较两个快照的关系集合。
// 返回按关系 ID 排序的差异列表；完全相同则返回空列表。
func Diff(left, right *model.Snapshot) []DiffEntry {
	lm := linkMap(left)
	rm := linkMap(right)
	var ids []string
	seen := map[string]bool{}
	for k := range lm {
		ids = append(ids, k)
		seen[k] = true
	}
	for k := range rm {
		if !seen[k] {
			ids = append(ids, k)
		}
	}
	sort.Strings(ids)

	var out []DiffEntry
	for _, id := range ids {
		l, hasL := lm[id]
		r, hasR := rm[id]
		switch {
		case hasL && !hasR:
			out = append(out, DiffEntry{CorrelationID: id, UpperBatchID: l.UpperBatchID, LowerBatchID: l.LowerBatchID, LeftStatus: l.Status, LeftDistance: fptr(l.Distance), Change: "removed"})
		case !hasL && hasR:
			out = append(out, DiffEntry{CorrelationID: id, UpperBatchID: r.UpperBatchID, LowerBatchID: r.LowerBatchID, RightStatus: r.Status, RightDistance: fptr(r.Distance), Change: "added"})
		default:
			change := "unchanged"
			if l.Status != r.Status {
				change = "status_changed"
			} else if l.Distance != r.Distance || l.PValue != r.PValue {
				change = "distance_changed"
			}
			out = append(out, DiffEntry{
				CorrelationID: id,
				UpperBatchID:  l.UpperBatchID,
				LowerBatchID:  l.LowerBatchID,
				LeftStatus:    l.Status,
				RightStatus:   r.Status,
				LeftDistance:  fptr(l.Distance),
				RightDistance: fptr(r.Distance),
				Change:        change,
			})
		}
	}
	return out
}

func linkMap(s *model.Snapshot) map[string]model.SnapshotLink {
	m := make(map[string]model.SnapshotLink, len(s.Links))
	for _, l := range s.Links {
		m[l.CorrelationID] = l
	}
	return m
}

func fptr(v float64) *float64 {
	return &v
}
