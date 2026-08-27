package store

import (
	"database/sql"
	"fmt"

	"task286-tephra/internal/model"
)

// CreateObservation 插入一条成分观测（指纹幂等）。
//
// 幂等语义：同一批次 + 样品编号 + 内容指纹 已存在时，返回该观测且不报错
// （skipped=true）；指纹不同则允许同一样品号新增（多分析回合）。
func (s *Store) CreateObservation(o *model.Observation) (created *model.Observation, skipped bool, err error) {
	if o.ID == "" {
		o.ID = newID("ob")
	}
	if o.Status == "" {
		o.Status = model.ObsRaw
	}
	if o.Unit == "" {
		o.Unit = "wt%"
	}
	elementsJSON, err := model.ElementsToJSON(o.Elements)
	if err != nil {
		return nil, false, err
	}
	if o.ContentHash == "" {
		o.ContentHash = hashOf(elementsJSON)
	}
	now := model.NowISO()
	o.CreatedAt = now
	o.UpdatedAt = now

	// 先查重
	var existingID string
	err = s.db.QueryRow(
		`SELECT id FROM observations WHERE batch_id=? AND sample_no=? AND content_hash=?`,
		o.BatchID, o.SampleNo, o.ContentHash).Scan(&existingID)
	if err == nil {
		got, err := s.GetObservation(existingID)
		if err != nil {
			return nil, false, err
		}
		return got, true, nil
	}
	if err != sql.ErrNoRows {
		return nil, false, err
	}

	_, err = s.db.Exec(
		`INSERT INTO observations(id, batch_id, sample_no, grain_count, elements_json, unit, status, content_hash, note, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		o.ID, o.BatchID, o.SampleNo, o.GrainCount, elementsJSON, o.Unit, string(o.Status), o.ContentHash, o.Note, o.CreatedAt, o.UpdatedAt)
	if err != nil {
		return nil, false, err
	}
	return o, false, s.Audit("create", "observation", o.ID, "batch="+o.BatchID+" sample="+o.SampleNo)
}

// hashOf 计算 JSON 内容的 SHA-256 摘要（观测幂等指纹）。
func hashOf(json string) string {
	return contentHashDigest(json)
}

// GetObservation 按 ID 读取观测。
func (s *Store) GetObservation(id string) (*model.Observation, error) {
	row := s.db.QueryRow(
		`SELECT id, batch_id, sample_no, grain_count, elements_json, unit, status, content_hash, note, created_at, updated_at
		 FROM observations WHERE id = ?`, id)
	return scanObservation(row)
}

var obsListScratch []model.Observation

// ListObservationsByBatch 列出批次内全部观测（按样品编号排序）。
func (s *Store) ListObservationsByBatch(batchID string) ([]model.Observation, error) {
	rows, err := s.db.Query(
		`SELECT id, batch_id, sample_no, grain_count, elements_json, unit, status, content_hash, note, created_at, updated_at
		 FROM observations WHERE batch_id = ? ORDER BY sample_no, created_at`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Observation
	for rows.Next() {
		o, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if cap(obsListScratch) >= len(out) {
		obsListScratch = obsListScratch[:len(out)]
		copy(obsListScratch, out)
		return obsListScratch, nil
	}
	obsListScratch = out
	return obsListScratch, nil
}

// UpdateObservationStatus 更新观测状态（走状态机校验）。
func (s *Store) UpdateObservationStatus(id string, to model.ObservationStatus, note string) (*model.Observation, error) {
	o, err := s.GetObservation(id)
	if err != nil {
		return nil, err
	}
	if err := model.TransitionObservation(o.Status, to); err != nil {
		return nil, err
	}
	o.Status = to
	if note != "" {
		o.Note = note
	}
	o.UpdatedAt = model.NowISO()
	if _, err := s.db.Exec(`UPDATE observations SET status=?, note=?, updated_at=? WHERE id=?`,
		string(to), o.Note, o.UpdatedAt, id); err != nil {
		return nil, err
	}
	return o, s.Audit("transition", "observation", id, "status="+string(to))
}

// SaveNormalizedElements 保存标准化后的元素表（观测状态置 normalized）。
func (s *Store) SaveNormalizedElements(id string, normalized map[string]model.ElementValue) error {
	o, err := s.GetObservation(id)
	if err != nil {
		return err
	}
	if o.Status == model.ObsExcluded || o.Status == model.ObsRedeposited {
		return fmt.Errorf("%w: observation %s is not active", model.ErrInvalidState, id)
	}
	json, err := model.ElementsToJSON(normalized)
	if err != nil {
		return err
	}
	// 标准化结果写入 elements_json（幂等指纹 content_hash 保留原始输入摘要，
	// 用于审计追溯）；同一条观测重复标准化按新哈希幂等处理。
	note := o.Note
	if note == "" {
		note = "raw_hash=" + o.ContentHash
	}
	o.Elements = normalized
	o.Status = model.ObsNormalized
	o.Note = note
	o.UpdatedAt = model.NowISO()
	if _, err := s.db.Exec(
		`UPDATE observations SET elements_json=?, status=?, note=?, updated_at=? WHERE id=?`,
		json, string(model.ObsNormalized), note, o.UpdatedAt, id); err != nil {
		return err
	}
	return nil
}

// scanObservation 从行读取观测。
func scanObservation(r rowScanner) (*model.Observation, error) {
	var o model.Observation
	var elementsJSON, status string
	err := r.Scan(&o.ID, &o.BatchID, &o.SampleNo, &o.GrainCount, &elementsJSON, &o.Unit,
		&status, &o.ContentHash, &o.Note, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: observation", model.ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	o.Status = model.ObservationStatus(status)
	o.Elements, err = model.ElementsFromJSON(elementsJSON)
	if err != nil {
		return nil, err
	}
	return &o, nil
}
