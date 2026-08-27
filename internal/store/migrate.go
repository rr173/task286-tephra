package store

import (
	"fmt"

	"task286-tephra/internal/model"
)

// schemaMigrations 记录 schema 版本。版本单调递增，禁止修改已发布的版本。
const schemaVersion = 1

// migrate 幂等建表并记录 schema 版本。
func (s *Store) migrate() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS batches (
			id           TEXT PRIMARY KEY,
			name         TEXT NOT NULL UNIQUE,
			site         TEXT NOT NULL DEFAULT '',
			formation    TEXT NOT NULL DEFAULT '',
			top_depth    REAL NOT NULL,
			bottom_depth REAL NOT NULL,
			depth_unit   TEXT NOT NULL DEFAULT 'm',
			status       TEXT NOT NULL DEFAULT 'collecting',
			description  TEXT NOT NULL DEFAULT '',
			created_at   TEXT NOT NULL,
			updated_at   TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS observations (
			id           TEXT PRIMARY KEY,
			batch_id     TEXT NOT NULL REFERENCES batches(id),
			sample_no    TEXT NOT NULL,
			grain_count  INTEGER NOT NULL DEFAULT 1,
			elements_json TEXT NOT NULL,
			unit         TEXT NOT NULL DEFAULT 'wt%',
			status       TEXT NOT NULL DEFAULT 'raw',
			content_hash TEXT NOT NULL,
			note         TEXT NOT NULL DEFAULT '',
			created_at   TEXT NOT NULL,
			updated_at   TEXT NOT NULL,
			UNIQUE(batch_id, sample_no, content_hash)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_obs_batch ON observations(batch_id)`,
		`CREATE TABLE IF NOT EXISTS correlation_links (
			id                TEXT PRIMARY KEY,
			upper_batch_id    TEXT NOT NULL REFERENCES batches(id),
			lower_batch_id    TEXT NOT NULL REFERENCES batches(id),
			status            TEXT NOT NULL DEFAULT 'candidate',
			distance          REAL NOT NULL DEFAULT 0,
			dof               INTEGER NOT NULL DEFAULT 0,
			p_value           REAL NOT NULL DEFAULT 0,
			compatible        INTEGER NOT NULL DEFAULT 0,
			strat_ok          INTEGER NOT NULL DEFAULT 0,
			strat_gap         REAL NOT NULL DEFAULT 0,
			strat_note        TEXT NOT NULL DEFAULT '',
			verdict           TEXT NOT NULL DEFAULT '',
			verdict_note      TEXT NOT NULL DEFAULT '',
			standard_version  TEXT NOT NULL DEFAULT '',
			created_at        TEXT NOT NULL,
			updated_at        TEXT NOT NULL,
			CHECK (upper_batch_id <> lower_batch_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_corr_upper ON correlation_links(upper_batch_id)`,
		`CREATE INDEX IF NOT EXISTS idx_corr_lower ON correlation_links(lower_batch_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_corr_pair ON correlation_links(upper_batch_id, lower_batch_id)`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id               TEXT PRIMARY KEY,
			name             TEXT NOT NULL UNIQUE,
			status           TEXT NOT NULL DEFAULT 'draft',
			standard_version TEXT NOT NULL,
			note             TEXT NOT NULL DEFAULT '',
			created_at       TEXT NOT NULL,
			published_at     TEXT,
			superseded_by    TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS snapshot_links (
			snapshot_id     TEXT NOT NULL REFERENCES snapshots(id),
			correlation_id  TEXT NOT NULL,
			upper_batch_id  TEXT NOT NULL,
			lower_batch_id  TEXT NOT NULL,
			status          TEXT NOT NULL,
			distance        REAL NOT NULL,
			p_value         REAL NOT NULL,
			strat_gap       REAL NOT NULL,
			PRIMARY KEY (snapshot_id, correlation_id)
		)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			action     TEXT NOT NULL,
			entity     TEXT NOT NULL,
			entity_id  TEXT NOT NULL,
			detail     TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("store: migrate stmt: %w", err)
		}
	}

	// 记录 schema 版本（幂等）
	var existing int
	err = tx.QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&existing)
	if err != nil {
		return err
	}
	if existing < schemaVersion {
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)`,
			schemaVersion, model.NowISO()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Audit 追加一条审计日志。
func (s *Store) Audit(action, entity, entityID, detail string) error {
	_, err := s.db.Exec(
		`INSERT INTO audit_log(action, entity, entity_id, detail, created_at) VALUES(?,?,?,?,?)`,
		action, entity, entityID, detail, model.NowISO())
	return err
}

// RecentAudits 返回最近 n 条审计日志（新到旧）。
func (s *Store) RecentAudits(n int) ([]AuditEntry, error) {
	if n <= 0 || n > 500 {
		n = 50
	}
	rows, err := s.db.Query(
		`SELECT id, action, entity, entity_id, detail, created_at FROM audit_log ORDER BY id DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Action, &e.Entity, &e.EntityID, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AuditEntry 描述一条审计日志。
type AuditEntry struct {
	ID        int64  `json:"id"`
	Action    string `json:"action"`
	Entity    string `json:"entity"`
	EntityID  string `json:"entity_id"`
	Detail    string `json:"detail"`
	CreatedAt string `json:"created_at"`
}
