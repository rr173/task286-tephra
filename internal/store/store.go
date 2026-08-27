// Package store 提供 SQLite 持久化实现。
//
// 使用纯 Go 驱动 modernc.org/sqlite（CGO_ENABLED=0 可构建）。
// 所有写入都经过事务；观测导入按内容指纹幂等；快照发布后数据不可变。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store 是数据访问层的聚合入口。
type Store struct {
	db *sql.DB
}

// Open 打开（必要时创建）SQLite 数据库并执行幂等迁移。
// path 为空时使用内存数据库（仅用于测试）。
func Open(path string) (*Store, error) {
	if path == "" {
		path = ":memory:"
	}
	if path != ":memory:" {
		if dir := filepath.Dir(path); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("store: mkdir %s: %w", dir, err)
			}
		}
	}
	dsn := path
	if path != ":memory:" {
		// WAL 模式 + busy_timeout，保证并发读写的安全与可靠性。
		dsn = fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(4) // SQLite WAL 支持并发读；写由数据库自身串行化
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: migrate: %w", err)
	}
	return s, nil
}

// Close 关闭数据库连接。
func (s *Store) Close() error {
	return s.db.Close()
}

// DB 暴露底层连接（仅限同包内部使用）。
func (s *Store) DB() *sql.DB {
	return s.db
}
