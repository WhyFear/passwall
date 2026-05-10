package repository

import (
	"fmt"
	"log"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"passwall/config"
	"passwall/internal/model"
)

// DB 全局数据库连接
var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB(dbConfig config.Database) (*gorm.DB, error) {
	var err error
	var dialector gorm.Dialector

	// 根据配置选择数据库驱动
	switch dbConfig.Driver {
	case "sqlite":
		dialector = sqlite.Open(dbConfig.DSN)
	case "postgres":
		dialector = postgres.Open(dbConfig.DSN)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", dbConfig.Driver)
	}

	// 连接数据库
	DB, err = gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// 配置连接池
	sqlDB, err := DB.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 对于SQLite，设置PRAGMA参数以提高并发性能
	if dbConfig.Driver == "sqlite" {
		// 设置WAL模式，提高并发性能
		DB.Exec("PRAGMA journal_mode = WAL;")
		// 设置busy_timeout，避免"database is locked"错误
		DB.Exec("PRAGMA busy_timeout = 5000;")
		// 设置同步模式为NORMAL，提高性能
		DB.Exec("PRAGMA synchronous = NORMAL;")
		// 设置缓存大小，减少磁盘I/O,20MB
		DB.Exec("PRAGMA cache_size = -20000;")

		// 使用GORM自动迁移表结构
		err = DB.AutoMigrate(
			&model.Proxy{},
			&model.Subscription{},
			&model.SubscriptionConfig{},
			&model.SpeedTestHistory{},
			&model.IPAddress{},
			&model.ProxyIPAddress{},
			&model.IPInfo{},
			&model.IPBaseInfo{},
			&model.IPUnlockInfo{},
			&model.SystemConfig{},
			&model.ShareConfig{},
		)
		if err != nil {
			return nil, err
		}
	} else if dbConfig.Driver == "postgres" {
		log.Println("使用PostgreSQL数据库，仅针对配置表执行自动迁移，避免干扰已有表结构...")
		// 仅迁移新增的或需要由GORM管理的配置类表
		err = DB.AutoMigrate(
			&model.SubscriptionConfig{},
			&model.SystemConfig{},
			&model.ShareConfig{},
		)
		if err != nil {
			return nil, err
		}
	}

	if err := ensurePerformanceIndexes(DB); err != nil {
		return nil, err
	}

	log.Println("数据库初始化成功")
	return DB, nil
}

func ensurePerformanceIndexes(db *gorm.DB) error {
	indexStatements := []string{
		"CREATE INDEX IF NOT EXISTS idx_speed_test_histories_proxy_created ON speed_test_histories (proxy_id, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_proxy_ip_addresses_proxy_latest_type ON proxy_ip_addresses (proxy_id, latest, ip_type)",
	}
	for _, statement := range indexStatements {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("ensure performance index failed: %w", err)
		}
	}
	return nil
}
