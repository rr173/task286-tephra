package store

import (
	"database/sql"
	"fmt"

	"task286-tephra/internal/model"
)

// CreateCorrelation 创建一个相关关系候选（upper 在上、lower 在下）。
// 同一 (upper, lower) 组合唯一；重复创建返回 ErrConflict。
func (s *Store) CreateCorrelation(c *model.Correlation) (*model.Correlation, error) {
	if c.ID == "" {
		c.ID = newID("co")
	}
	if c.UpperBatchID == c.LowerBatchID {
		return nil, fmt.Errorf("%w: cannot correlate a batch with itself", model.ErrInvalidInput)
	}
	if c.Status == "" {
		c.Status = model.CorrCandidate
	}
	now := model.NowISO()
	c.CreatedAt = now
	c.UpdatedAt = now
	compatible := boolToInt(c.Compatible)
	stratOK := boolToInt(c.StratOK)
	_, err := s.db.Exec(
		`INSERT INTO correlation_links(id, upper_batch_id, lower_batch_id, status, distance, dof, p_value, compatible, strat_ok, strat_gap, strat_note, verdict, verdict_note, standard_version, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.ID, c.UpperBatchID, c.LowerBatchID, string(c.Status), c.Distance, c.DOF, c.PValue,
		compatible, stratOK, c.StratGap, c.StratNote, c.Verdict, c.VerdictNote, c.StandardVersion,
		c.CreatedAt, c.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: correlation %s/%s already exists", model.ErrConflict, c.UpperBatchID, c.LowerBatchID)
		}
		return nil, err
	}
	return c, s.Audit("create", "correlation", c.ID, fmt.Sprintf("%s~%s", c.UpperBatchID, c.LowerBatchID))
}

// GetCorrelation 按 ID 读取相关关系。
func (s *Store) GetCorrelation(id string) (*model.Correlation, error) {
	row := s.db.QueryRow(
		`SELECT id, upper_batch_id, lower_batch_id, status, distance, dof, p_value, compatible, strat_ok, strat_gap, strat_note, verdict, verdict_note, standard_version, created_at, updated_at
		 FROM correlation_links WHERE id = ?`, id)
	return scanCorrelation(row)
}

// FindCorrelation 按批次对查询已存在的相关关系。
func (s *Store) FindCorrelation(upperID, lowerID string) (*model.Correlation, error) {
	row := s.db.QueryRow(
		`SELECT id, upper_batch_id, lower_batch_id, status, distance, dof, p_value, compatible, strat_ok, strat_gap, strat_note, verdict, verdict_note, standard_version, created_at, updated_at
		 FROM correlation_links WHERE upper_batch_id=? AND lower_batch_id=?`, upperID, lowerID)
	return scanCorrelation(row)
}

// ListCorrelations 列出全部相关关系（按创建时间倒序）。
func (s *Store) ListCorrelations() ([]model.Correlation, error) {
	rows, err := s.db.Query(
		`SELECT id, upper_batch_id, lower_batch_id, status, distance, dof, p_value, compatible, strat_ok, strat_gap, strat_note, verdict, verdict_note, standard_version, created_at, updated_at
		 FROM correlation_links ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Correlation
	for rows.Next() {
		c, err := scanCorrelation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// ListCorrelationsByBatch 列出某批次参与的相关关系。
func (s *Store) ListCorrelationsByBatch(batchID string) ([]model.Correlation, error) {
	rows, err := s.db.Query(
		`SELECT id, upper_batch_id, lower_batch_id, status, distance, dof, p_value, compatible, strat_ok, strat_gap, strat_note, verdict, verdict_note, standard_version, created_at, updated_at
		 FROM correlation_links WHERE upper_batch_id=? OR lower_batch_id=? ORDER BY created_at DESC`,
		batchID, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Correlation
	for rows.Next() {
		c, err := scanCorrelation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// SaveCorrelationResult 保存指纹/层位计算结果（仅候选或相符/冲突状态可写）。
func (s *Store) SaveCorrelationResult(c *model.Correlation) error {
	if c.Status == model.CorrConfirmed || c.Status == model.CorrRejected {
		return fmt.Errorf("%w: adjudicated correlation cannot be recomputed", model.ErrSealed)
	}
	compatible := boolToInt(c.Compatible)
	stratOK := boolToInt(c.StratOK)
	c.UpdatedAt = model.NowISO()
	_, err := s.db.Exec(
		`UPDATE correlation_links SET status=?, distance=?, dof=?, p_value=?, compatible=?, strat_ok=?, strat_gap=?, strat_note=?, standard_version=?, updated_at=? WHERE id=? AND verdict=?`,
		string(c.Status), c.Distance, c.DOF, c.PValue, compatible, stratOK, c.StratGap, c.StratNote,
		c.StandardVersion, c.UpdatedAt, c.ID, c.Verdict)
	if err != nil {
		return err
	}
	return s.Audit("recompute", "correlation", c.ID, fmt.Sprintf("D2=%.3f p=%.4f", c.Distance, c.PValue))
}

// AdjudicateCorrelation 裁决相关关系（确认/否决）。
func (s *Store) AdjudicateCorrelation(c *model.Correlation) error {
	c.UpdatedAt = model.NowISO()
	_, err := s.db.Exec(
		`UPDATE correlation_links SET status=?, verdict=?, verdict_note=?, updated_at=? WHERE id=?`,
		string(c.Status), c.Verdict, c.VerdictNote, c.UpdatedAt, c.ID)
	if err != nil {
		return err
	}
	return s.Audit("adjudicate", "correlation", c.ID, "verdict="+c.Verdict)
}

// ReopenCorrelation 把已裁决/候选关系重置回 candidate（标准版本升级后重算）。
func (s *Store) ReopenCorrelation(id string) error {
	c, err := s.GetCorrelation(id)
	if err != nil {
		return err
	}
	if err := correlateReopen(c); err != nil {
		return err
	}
	c.UpdatedAt = model.NowISO()
	_, err = s.db.Exec(
		`UPDATE correlation_links SET status=?, verdict=?, verdict_note=?, updated_at=? WHERE id=?`,
		string(c.Status), c.Verdict, c.VerdictNote, c.UpdatedAt, id)
	if err != nil {
		return err
	}
	return s.Audit("reopen", "correlation", id, "for recompute")
}

// correlateReopen 重置状态（复用 correlate 包的语义，避免循环导入）。
func correlateReopen(c *model.Correlation) error {
	if c.Status == model.CorrConfirmed || c.Status == model.CorrRejected {
		c.Status = model.CorrCandidate
		c.Verdict = ""
		c.VerdictNote = ""
	}
	return nil
}

func scanCorrelation(r rowScanner) (*model.Correlation, error) {
	var c model.Correlation
	var status string
	var compatible, stratOK int
	err := r.Scan(&c.ID, &c.UpperBatchID, &c.LowerBatchID, &status, &c.Distance, &c.DOF, &c.PValue,
		&compatible, &stratOK, &c.StratGap, &c.StratNote, &c.Verdict, &c.VerdictNote,
		&c.StandardVersion, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: correlation", model.ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	c.Status = model.CorrelationStatus(status)
	c.Compatible = compatible == 1
	c.StratOK = stratOK == 1
	return &c, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
