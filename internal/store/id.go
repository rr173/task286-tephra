package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// newID 生成一个短随机 ID（前缀 + 时间戳 + 随机字节）。
// 用于所有实体的主键；随机部分保证并发环境下的唯一性。
func newID(prefix string) string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		// 随机源失败时退化为时间戳，仍满足唯一性（单进程内）。
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%s-%s", prefix, time.Now().Format("20060102150405"), hex.EncodeToString(buf))
}

// isUniqueViolation 判断 SQLite 错误是否为 UNIQUE 约束冲突。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, m := range []string{"UNIQUE constraint failed", "constraint failed: UNIQUE"} {
		if contains(msg, m) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// contentHashDigest 计算文本内容的 SHA-256 十六进制摘要。
func contentHashDigest(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
