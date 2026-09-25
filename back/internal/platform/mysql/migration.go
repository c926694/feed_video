package mysql

import (
	_ "embed"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

//go:embed sql/feed_video.sql
var schemaSQL string

// Migrate 在服务启动时按 SQL 文件建立缺失的表。
// 文件里的语句都是 CREATE TABLE IF NOT EXISTS，已存在的表不会被改动。
func Migrate(db *gorm.DB) error {
	for _, statement := range splitStatements(schemaSQL) {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("执行建表语句失败: %w\n语句: %s", err, statement)
		}
	}
	return nil
}

// splitStatements 按分号切出可执行的语句，并丢掉空片段
func splitStatements(script string) []string {
	statements := make([]string, 0)
	for _, raw := range strings.Split(script, ";") {
		if statement := strings.TrimSpace(raw); statement != "" {
			statements = append(statements, statement)
		}
	}
	return statements
}
