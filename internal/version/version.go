// Package version 提供构建与运行时的版本信息。
package version

// 构建信息（构建时可通过 -ldflags 覆盖）。
var (
	// Version 是服务版本号。
	Version = "1.0.0"
	// GoVersion 是编译所用 Go 工具链版本。
	GoVersion = "1.26.3"
	// SQLiteVersion 是运行时 SQLite 版本（由 modernc 驱动实现）。
	SQLiteVersion = "3.46.1"
	// SQLiteDriver 是使用的驱动标识。
	SQLiteDriver = "modernc.org/sqlite"
)

// ComponentVersions 返回组件版本锁定信息（与 component-versions.json 对齐）。
func ComponentVersions() map[string]string {
	return map[string]string{
		"go":      GoVersion,
		"sqlite":  SQLiteVersion,
		"service": Version,
	}
}
