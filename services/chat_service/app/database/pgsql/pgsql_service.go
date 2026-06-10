package pgsql

import (
	"fmt"
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	config "chat_service/app/config"
	"chat_service/app/database/pgsql/model"
	"chat_service/app/database/pgsql/query"
)

// DBManager 数据库管理器结构体
type DBManager struct {
	db *gorm.DB
}

// 使用单例模式确保只有一个数据库实例
var (
	instance *DBManager
	once     sync.Once
)

// GetDBManager 获取数据库管理器实例
func GetDBManager() *DBManager {
	once.Do(func() {
		instance = &DBManager{}
	})
	return instance
}

// Connect 连接到PostgreSQL数据库
func (manager *DBManager) Connect() error {
	// 从配置文件获取数据库连接信息
	host := config.DataBaseConfig.Host
	port := config.DataBaseConfig.Port
	user := config.DataBaseConfig.User
	password := config.DataBaseConfig.Password
	dbname := config.DataBaseConfig.DBName

	// 构建DSN (Data Source Name)
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		host, user, password, dbname, port, config.DataBaseConfig.SSLMode, config.DataBaseConfig.TimeZone)

	// Gorm框架提供的用于初始化数据库连接的主要函数
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 获取通用数据库对象 sql.DB 以配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	// 设置空闲连接池中连接的最大数量
	sqlDB.SetMaxIdleConns(10)
	// 设置数据库连接最大生存时间
	sqlDB.SetConnMaxLifetime(0)
	// 设置打开数据库连接的最大数量
	sqlDB.SetMaxOpenConns(100)

	manager.db = db

	// 设置 GEN 的默认数据库连接
	query.SetDefault(db)

	return nil
}

// Close 关闭数据库连接
func (manager *DBManager) Close() error {
	if manager.db != nil {
		sqlDB, err := manager.db.DB()
		if err != nil {
			return fmt.Errorf("failed to get database instance: %w", err)
		}
		return sqlDB.Close()
	}
	return nil
}

// GetDB 获取数据库实例
func (manager *DBManager) GetDB() *gorm.DB {
	return manager.db
}

// Initialize 初始化数据库，自动创建表结构
func (manager *DBManager) Initialize() error {
	if manager.db == nil {
		return fmt.Errorf("database not connected")
	}

	// 1. 自动迁移表结构
	if err := model.AutoMigrate(manager.db); err != nil {
		return fmt.Errorf("failed to auto migrate: %w", err)
	}

	// 2. 初始化 Sequence（如果不存在则创建）
	sequences := []string{
		"group_id_seq",
	}

	for _, seq := range sequences {
		sql := fmt.Sprintf(`
			CREATE SEQUENCE IF NOT EXISTS %s
			START WITH 1
			INCREMENT BY 1
			NO MAXVALUE
			NO CYCLE;
		`, seq)
		if err := manager.db.Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create sequence %s: %w", seq, err)
		}
	}

	return nil
}
