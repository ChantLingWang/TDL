package database

import (
	"fmt"
	"log"
	"os"
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"chant/user_service/app/core"
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

// GetDBManager 获取数据库管理器实例（单例模式）
func GetDBManager() *DBManager {
	once.Do(func() {
		instance = &DBManager{}
	})
	return instance
}

// Connect 连接到PostgreSQL数据库
func (manager *DBManager) Connect() error {
	// 从配置文件获取数据库连接信息
	host := core.DataBaseConfig.Host
	port := core.DataBaseConfig.Port
	user := core.DataBaseConfig.User
	password := core.DataBaseConfig.Password
	dbname := core.DataBaseConfig.DBName

	// 构建DSN (Data Source Name)
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		host, user, password, dbname, port, core.DataBaseConfig.SSLMode, core.DataBaseConfig.TimeZone)
	
	// 配置GORM日志级别
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			LogLevel: logger.Info, // Log level
			Colorful: true,        // 彩色打印
		},
	)

	// 连接数据库
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 获取通用数据库对象 sql.DB 以配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	// 设置连接池
	// 设置空闲连接池中连接的最大数量
	sqlDB.SetMaxIdleConns(10)
	// 设置数据库连接最大生存时间
	sqlDB.SetConnMaxLifetime(0)
	// 设置打开数据库连接的最大数量
	sqlDB.SetMaxOpenConns(100)

	manager.db = db
	log.Println("Successfully connected to the database")
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
	
	return AutoMigrate(manager.db)
}
