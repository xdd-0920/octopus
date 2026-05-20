package migrate

import (
	"fmt"

	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 4,
		Up:      addRelayLogIndexes,
	})
}

// 004: add indexes to relay_logs table for better query performance
func addRelayLogIndexes(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	dialect := db.Dialector.Name()

	// 添加索引的辅助函数
	addIndex := func(table, indexName string, columns []string) error {
		// 检查索引是否存在
		var exists bool
		switch dialect {
		case "sqlite":
			var count int
			db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", indexName).Scan(&count)
			exists = count > 0
		case "mysql":
			var count int
			db.Raw("SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?", table, indexName).Scan(&count)
			exists = count > 0
		case "postgres":
			var count int
			db.Raw("SELECT COUNT(*) FROM pg_indexes WHERE tablename = ? AND indexname = ?", table, indexName).Scan(&count)
			exists = count > 0
		default:
			// 默认尝试创建，如果已存在会报错
			exists = false
		}

		if exists {
			return nil
		}

		// 创建索引
		var sql string
		switch dialect {
		case "sqlite":
			sql = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)", indexName, table, joinColumns(columns))
		case "mysql":
			sql = fmt.Sprintf("CREATE INDEX %s ON `%s` (%s)", indexName, table, joinColumns(columns))
		case "postgres":
			sql = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)", indexName, table, joinColumns(columns))
		default:
			sql = fmt.Sprintf("CREATE INDEX %s ON %s (%s)", indexName, table, joinColumns(columns))
		}

		return db.Exec(sql).Error
	}

	// 添加 time 字段索引（用于时间范围查询和排序）
	if err := addIndex("relay_logs", "idx_relay_logs_time", []string{"time"}); err != nil {
		return fmt.Errorf("failed to add time index: %w", err)
	}

	// 添加 id 字段索引（用于主键查询和排序）
	if err := addIndex("relay_logs", "idx_relay_logs_id", []string{"id"}); err != nil {
		return fmt.Errorf("failed to add id index: %w", err)
	}

	return nil
}

func joinColumns(columns []string) string {
	result := ""
	for i, col := range columns {
		if i > 0 {
			result += ", "
		}
		result += col
	}
	return result
}