package mysql

import (
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql" // 注册 mysql 数据库驱动
	"github.com/golang-migrate/migrate/v4/source/iofs"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate 按版本执行 migrations 目录下的 SQL 迁移，每个迁移只执行一次，
// 已执行的版本记录在 schema_migrations 表里。迁移使用独立连接，完成后关闭，
// 不影响 gorm 的连接池。
func Migrate(db *gorm.DB) error {
	dialector, ok := db.Dialector.(*gormmysql.Dialector)
	if !ok {
		return errors.New("当前数据库方言不是 mysql")
	}
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("加载迁移文件失败: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, "mysql://"+dialector.DSN)
	if err != nil {
		return fmt.Errorf("初始化迁移失败: %w", err)
	}
	defer func() { _, _ = m.Close() }()
	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("执行迁移失败: %w", err)
	}
	return nil
}
