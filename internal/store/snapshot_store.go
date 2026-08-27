package store

import (
	"database/sql"
	"fmt"

	"task286-tephra/internal/model"
)

// CreateSnapshot 创建快照（草稿），并把链接行持久化。
func (s *Store) CreateSnapshot(snap *model.Snapshot) (*model.Snapshot, error) {
	if snap.ID == "" {
		snap.ID = newID("sn")
	}
	if snap.Status == "" {
		snap.Status = model.SnapDraft
	}
	now := model.NowISO()
	snap.CreatedAt = now
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.Exec(
		`INSERT INTO snapshots(id, name, status, standard_version, note, created_at, published_at, superseded_by)
		 VALUES(?,?,?,?,?,?,?,?)`,
		snap.ID, snap.Name, string(snap.Status), snap.StandardVersion, snap.Note, snap.CreatedAt, "", "")
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: snapshot name %q already exists", model.ErrConflict, snap.Name)
		}
		return nil, err
	}
	for _, link := range snap.Links {
		if _, err := tx.Exec(
			`INSERT INTO snapshot_links(snapshot_id, correlation_id, upper_batch_id, lower_batch_id, status, distance, p_value, strat_gap)
			 VALUES(?,?,?,?,?,?,?,?)`,
			snap.ID, link.CorrelationID, link.UpperBatchID, link.LowerBatchID, link.Status,
			link.Distance, link.PValue, link.StratGap); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return snap, s.Audit("create", "snapshot", snap.ID, "name="+snap.Name+" links="+itoa(len(snap.Links)))
}

// GetSnapshot 按 ID 读取快照（含链接行）。
func (s *Store) GetSnapshot(id string) (*model.Snapshot, error) {
	snap, err := s.scanSnapshotRow(id)
	if err != nil {
		return nil, err
	}
	if err := s.loadSnapshotLinks(snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// ListSnapshots 列出全部快照（不含链接，按创建时间倒序）。
func (s *Store) ListSnapshots() ([]model.Snapshot, error) {
	rows, err := s.db.Query(
		`SELECT id, name, status, standard_version, note, created_at, published_at, superseded_by
		 FROM snapshots ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Snapshot
	for rows.Next() {
		snap, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *snap)
	}
	return out, rows.Err()
}

// PublishSnapshot 发布快照（草稿 → published），固定不可变。
func (s *Store) PublishSnapshot(id string) (*model.Snapshot, error) {
	snap, err := s.GetSnapshot(id)
	if err != nil {
		return nil, err
	}
	if err := snapshotPublish(snap); err != nil {
		return nil, err
	}
	_, err = s.db.Exec(`UPDATE snapshots SET status=?, published_at=? WHERE id=?`,
		string(model.SnapPublished), snap.PublishedAt, id)
	if err != nil {
		return nil, err
	}
	// 纳入快照的关系的参与批次置为已发布（走状态机校验；sealed 批次保持原状）
	for _, link := range snap.Links {
		if b, err := s.GetBatch(link.UpperBatchID); err == nil && b.Status == model.BatchReady {
			_, _ = s.UpdateBatchStatus(link.UpperBatchID, model.BatchPublished)
		}
		if b, err := s.GetBatch(link.LowerBatchID); err == nil && b.Status == model.BatchReady {
			_, _ = s.UpdateBatchStatus(link.LowerBatchID, model.BatchPublished)
		}
	}
	return snap, s.Audit("publish", "snapshot", id, "links="+itoa(len(snap.Links)))
}

// SupersedeSnapshot 用新快照替代旧快照。
func (s *Store) SupersedeSnapshot(oldID, newID string) (*model.Snapshot, error) {
	oldSnap, err := s.GetSnapshot(oldID)
	if err != nil {
		return nil, err
	}
	newSnap, err := s.GetSnapshot(newID)
	if err != nil {
		return nil, err
	}
	if err := snapshotSupersede(oldSnap, newSnap); err != nil {
		return nil, err
	}
	_, err = s.db.Exec(`UPDATE snapshots SET status=?, superseded_by=?, created_at=? WHERE id=?`,
		string(model.SnapSuperseded), newID, oldSnap.CreatedAt, oldID)
	if err != nil {
		return nil, err
	}
	return oldSnap, s.Audit("supersede", "snapshot", oldID, "replaced_by="+newID)
}

// snapshotPublish 复用 snapshot 包的发布校验（避免循环导入）。
func snapshotPublish(snap *model.Snapshot) error {
	if snap.Status != model.SnapDraft {
		return fmt.Errorf("%w: only draft snapshots can be published", model.ErrInvalidState)
	}
	snap.Status = model.SnapPublished
	snap.PublishedAt = model.NowISO()
	return nil
}

// snapshotSupersede 复用 snapshot 包的替代校验。
func snapshotSupersede(oldSnap, newSnap *model.Snapshot) error {
	if oldSnap.Status != model.SnapPublished {
		return fmt.Errorf("%w: only published snapshots can be superseded", model.ErrInvalidState)
	}
	if newSnap.Status != model.SnapPublished {
		return fmt.Errorf("%w: replacement must be published", model.ErrInvalidState)
	}
	oldSnap.Status = model.SnapSuperseded
	oldSnap.SupersededBy = newSnap.ID
	return nil
}

// IsCorrelationFrozenInSnapshot 判断某关系是否已被任何冻结快照（发布/替代）纳入。
// 用于阻止对已入快照关系的数据修改。
func (s *Store) IsCorrelationFrozenInSnapshot(correlationID string) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM snapshot_links sl
		 JOIN snapshots sp ON sp.id = sl.snapshot_id
		 WHERE sl.correlation_id = ? AND sp.status IN (?, ?)`,
		correlationID, string(model.SnapPublished), string(model.SnapSuperseded)).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// GetSnapshotByName 按名称读取快照。
func (s *Store) GetSnapshotByName(name string) (*model.Snapshot, error) {
	snap, err := s.scanSnapshotRowByName(name)
	if err != nil {
		return nil, err
	}
	if err := s.loadSnapshotLinks(snap); err != nil {
		return nil, err
	}
	return snap, nil
}

func (s *Store) scanSnapshotRow(id string) (*model.Snapshot, error) {
	row := s.db.QueryRow(
		`SELECT id, name, status, standard_version, note, created_at, published_at, superseded_by
		 FROM snapshots WHERE id = ?`, id)
	return scanSnapshot(row)
}

func (s *Store) scanSnapshotRowByName(name string) (*model.Snapshot, error) {
	row := s.db.QueryRow(
		`SELECT id, name, status, standard_version, note, created_at, published_at, superseded_by
		 FROM snapshots WHERE name = ?`, name)
	return scanSnapshot(row)
}

func (s *Store) loadSnapshotLinks(snap *model.Snapshot) error {
	rows, err := s.db.Query(
		`SELECT correlation_id, upper_batch_id, lower_batch_id, status, distance, p_value, strat_gap
		 FROM snapshot_links WHERE snapshot_id = ? ORDER BY correlation_id`, snap.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var l model.SnapshotLink
		if err := rows.Scan(&l.CorrelationID, &l.UpperBatchID, &l.LowerBatchID, &l.Status,
			&l.Distance, &l.PValue, &l.StratGap); err != nil {
			return err
		}
		snap.Links = append(snap.Links, l)
	}
	return rows.Err()
}

func scanSnapshot(r rowScanner) (*model.Snapshot, error) {
	var s model.Snapshot
	var status string
	var publishedAt, supersededBy sql.NullString
	err := r.Scan(&s.ID, &s.Name, &status, &s.StandardVersion, &s.Note, &s.CreatedAt,
		&publishedAt, &supersededBy)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: snapshot", model.ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	s.Status = model.SnapshotStatus(status)
	if publishedAt.Valid {
		s.PublishedAt = publishedAt.String
	}
	if supersededBy.Valid {
		s.SupersededBy = supersededBy.String
	}
	return &s, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
