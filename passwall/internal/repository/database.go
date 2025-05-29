package repository

import (
	"fmt"
	"log"
	"time"

	"github.com/glebarez/sqlite"
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

	// 自动迁移表结构
	err = DB.AutoMigrate(
		&model.Proxy{},
		&model.Subscription{},
		&model.SpeedTestHistory{},
	)
	if err != nil {
		return nil, err
	}

	log.Println("数据库初始化成功")
	return DB, nil
}

// GetDB 获取数据库连接
func GetDB() *gorm.DB {
	return DB
}
