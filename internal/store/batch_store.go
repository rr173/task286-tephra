package store

import (
	"database/sql"
	"fmt"
	"sync"

	"task286-tephra/internal/model"
)

// CreateBatch 插入一个新灰层批次。
// 名称全局唯一（UNIQUE 约束），冲突时返回 ErrConflict。
func (s *Store) CreateBatch(b *model.Batch) error {
	if b.ID == "" {
		b.ID = newID("bt")
	}
	if b.DepthUnit == "" {
		b.DepthUnit = "m"
	}
	if b.Status == "" {
		b.Status = model.BatchCollecting
	}
	if !model.ValidDepthRange(b.TopDepth, b.BottomDepth) {
		return fmt.Errorf("%w: top=%.3f bottom=%.3f", model.ErrInvertedRange, b.TopDepth, b.BottomDepth)
	}
	now := model.NowISO()
	b.CreatedAt = now
	b.UpdatedAt = now
	_, err := s.db.Exec(
		`INSERT INTO batches(id, name, site, formation, top_depth, bottom_depth, depth_unit, status, description, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		b.ID, b.Name, b.Site, b.Formation, b.TopDepth, b.BottomDepth, b.DepthUnit, string(b.Status), b.Description, b.CreatedAt, b.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: batch name %q already exists", model.ErrConflict, b.Name)
		}
		return err
	}
	return s.Audit("create", "batch", b.ID, "name="+b.Name)
}

// GetBatch 按 ID 读取批次；不存在返回 ErrNotFound。
func (s *Store) GetBatch(id string) (*model.Batch, error) {
	row := s.db.QueryRow(
		`SELECT id, name, site, formation, top_depth, bottom_depth, depth_unit, status, description, created_at, updated_at
		 FROM batches WHERE id = ?`, id)
	return scanBatch(row)
}

// GetBatchByName 按名称读取批次。
func (s *Store) GetBatchByName(name string) (*model.Batch, error) {
	row := s.db.QueryRow(
		`SELECT id, name, site, formation, top_depth, bottom_depth, depth_unit, status, description, created_at, updated_at
		 FROM batches WHERE name = ?`, name)
	return scanBatch(row)
}

var (
	batchListMu    sync.RWMutex
	batchListCache []model.Batch
	batchListOK    bool
)

// ListBatches 列出全部批次（按创建时间倒序），含活跃观测数。
func (s *Store) ListBatches() ([]model.Batch, error) {
	batchListMu.RLock()
	if batchListOK {
		out := batchListCache
		batchListMu.RUnlock()
		return out, nil
	}
	batchListMu.RUnlock()
	rows, err := s.db.Query(
		`SELECT b.id, b.name, b.site, b.formation, b.top_depth, b.bottom_depth, b.depth_unit, b.status, b.description, b.created_at, b.updated_at,
		        (SELECT COUNT(*) FROM observations o WHERE o.batch_id = b.id AND o.status <> ?) AS obs_count
		 FROM batches b ORDER BY b.created_at DESC`, string(model.ObsExcluded))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Batch
	for rows.Next() {
		var b model.Batch
		var status string
		if err := rows.Scan(&b.ID, &b.Name, &b.Site, &b.Formation, &b.TopDepth, &b.BottomDepth, &b.DepthUnit,
			&status, &b.Description, &b.CreatedAt, &b.UpdatedAt, &b.ObservationCount); err != nil {
			return nil, err
		}
		b.Status = model.BatchStatus(status)
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	batchListMu.Lock()
	batchListCache = out
	batchListOK = true
	batchListMu.Unlock()
	return out, nil
}

// UpdateBatchStatus 更新批次状态（走状态机校验）。
func (s *Store) UpdateBatchStatus(id string, to model.BatchStatus) (*model.Batch, error) {
	b, err := s.GetBatch(id)
	if err != nil {
		return nil, err
	}
	if err := model.TransitionBatch(b.Status, to); err != nil {
		return nil, err
	}
	b.Status = to
	b.UpdatedAt = model.NowISO()
	if _, err := s.db.Exec(`UPDATE batches SET status=?, updated_at=? WHERE id=?`, string(to), b.UpdatedAt, id); err != nil {
		return nil, err
	}
	return b, s.Audit("transition", "batch", id, "status="+string(to))
}

// SetBatchReview 把批次置为需复核状态（观测异常时自动触发）。
func (s *Store) SetBatchReview(id string) error {
	b, err := s.GetBatch(id)
	if err != nil {
		return err
	}
	if b.Status == model.BatchReady || b.Status == model.BatchReview {
		return s.transitionStatus("batch", id, model.BatchReview)
	}
	return nil
}

// transitionStatus 免查询直接执行状态迁移（供内部批量流程使用）。
func (s *Store) transitionStatus(entity, id string, to interface{ String() string }) error {
	_, err := s.db.Exec(fmt.Sprintf(`UPDATE %s SET status=?, updated_at=? WHERE id=?`, entity), to.String(), model.NowISO(), id)
	return err
}

// CountActiveObservations 统计批次内活跃（非排除）观测数。
func (s *Store) CountActiveObservations(batchID string) (int, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM observations WHERE batch_id=? AND status <> ?`, batchID, string(model.ObsExcluded)).Scan(&n)
	return n, err
}

// scanBatch 从行读取批次。
type rowScanner interface{ Scan(dest ...any) error }

func scanBatch(r rowScanner) (*model.Batch, error) {
	var b model.Batch
	var status string
	err := r.Scan(&b.ID, &b.Name, &b.Site, &b.Formation, &b.TopDepth, &b.BottomDepth, &b.DepthUnit,
		&status, &b.Description, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: batch", model.ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	b.Status = model.BatchStatus(status)
	return &b, nil
}
