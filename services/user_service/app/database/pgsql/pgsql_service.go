package database

import (
	"fmt"
	"log"
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

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
	host := core.DataBaseConfig.Host
	port := core.DataBaseConfig.Port
	user := core.DataBaseConfig.User
	password := core.DataBaseConfig.Password
	dbname := core.DataBaseConfig.DBName

	// 构建DSN (Data Source Name)
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		host, user, password, dbname, port, core.DataBaseConfig.SSLMode, core.DataBaseConfig.TimeZone)
	
	// Gorm框架提供的用于初始化数据库连接的主要函数，接受两个参数，
	// 第一个参数是数据库驱动的Dialector，第二个参数是Gorm配置选项
	// 该函数返回两个值：
	// 第一个值是 *gorm.DB 类型的数据库实例，用于执行数据库操作
	// 第二个值是 err
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
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
// 注意sqlDB和db的区别
// sqlDB是 *sql.DB 类型，用于执行原始的SQL查询和操作
// db是 *gorm.DB 类型，提供了GORM框架的所有功能，包括ORM映射、事务管理等
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
