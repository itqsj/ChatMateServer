package config

import (
	"fmt"
	"os"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"server/global"
)

const mysqlDSNEnv = "MYSQL_DSN"

// MySQLConfig 保存 MySQL 数据库连接配置。
type MySQLConfig struct {
	DSN string
}

// LoadMySQLConfig 从环境变量读取 MySQL 配置。
func LoadMySQLConfig() MySQLConfig {
	LoadEnv()

	return MySQLConfig{
		DSN: strings.TrimSpace(os.Getenv(mysqlDSNEnv)),
	}
}

// InitMySQL 初始化 MySQL 连接并执行自动创表。
func InitMySQL() error {
	cfg := LoadMySQLConfig()
	if cfg.DSN == "" {
		// 当前没有业务表，未配置 MySQL 时允许服务正常启动。
		return nil
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return err
	}

	global.DB = db
	return AutoCreateTables(db)
}

// AutoCreateTables 自动创建或更新数据库表结构。
func AutoCreateTables(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("mysql db is nil")
	}

	models := []interface{}{}

	// 暂时没有业务表，后续新增模型后统一放到 models 列表里。
	if len(models) == 0 {
		return nil
	}

	return db.AutoMigrate(models...)
}
