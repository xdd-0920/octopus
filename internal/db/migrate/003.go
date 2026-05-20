package migrate

import (
	"fmt"

	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 3,
		Up:      addCachedTokensToRelayLog,
	})
}

// 003: add cached_tokens column to relay_logs table
func addCachedTokensToRelayLog(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	dialect := db.Dialector.Name()

	// 检查列是否存在
	hasColumn := func(table, column string) bool {
		switch dialect {
		case "sqlite":
			var name string
			db.Raw("SELECT name FROM pragma_table_info(?) WHERE name = ? LIMIT 1", table, column).Scan(&name)
			return name == column
		case "mysql":
			var count int64
			db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?", table, column).Scan(&count)
			return count > 0
		case "postgres":
			var count int64
			db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ?", table, column).Scan(&count)
			return count > 0
		default:
			return db.Migrator().HasColumn(table, column)
		}
	}

	// 如果列已存在，跳过
	if hasColumn("relay_logs", "cached_tokens") {
		return nil
	}

	// 添加 cached_tokens 列
	var sql string
	switch dialect {
	case "sqlite":
		sql = "ALTER TABLE relay_logs ADD COLUMN cached_tokens INTEGER DEFAULT 0"
	case "mysql":
		sql = "ALTER TABLE `relay_logs` ADD COLUMN `cached_tokens` INT DEFAULT 0"
	case "postgres":
		sql = "ALTER TABLE relay_logs ADD COLUMN cached_tokens INTEGER DEFAULT 0"
	default:
		sql = "ALTER TABLE relay_logs ADD COLUMN cached_tokens INTEGER DEFAULT 0"
	}

	if err := db.Exec(sql).Error; err != nil {
		return fmt.Errorf("failed to add cached_tokens column to relay_logs: %w", err)
	}

	return nil
}
