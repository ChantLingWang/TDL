package main

import (
	"log"

	"github.com/gin-gonic/gin"
	v1 "chant/user_service/app/api/v1"
	"chant/user_service/app/database/pgsql"
	"chant/user_service/app/database/mongodb"
)

// initPostgreSQL 初始化PostgreSQL数据库连接
func initPostgreSQL() {
	dbManager := database.GetDBManager()
	if err := dbManager.Connect(); err != nil {
		log.Fatalf("Failed to connect to PostgreSQL database: %v", err)
	}
	defer dbManager.Close()

	// 初始化数据库表结构
	if err := dbManager.Initialize(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
}

// initMongoDB 初始化MongoDB数据库连接
func initMongoDB() {
	mongoManager := mongodb.GetMongoDBManager()
	if err := mongoManager.Connect(); err != nil {
		log.Fatalf("Failed to connect to MongoDB database: %v", err)
	}
	defer mongoManager.Close()
}

// startServer 启动HTTP服务器
func startServer() {
	// 初始化Gin引擎
	engine := gin.Default()

	// 注册用户相关的路由
	v1.RegisterUserRoutes(engine)

	// 启动HTTP服务器
	log.Println("Starting server on :8080")
	if err := engine.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// initDatabases 初始化所有数据库连接
func initDatabases() {
	// 初始化PostgreSQL数据库连接
	initPostgreSQL()
	// 初始化MongoDB数据库连接
	initMongoDB()
}


func main() {
	// 初始化数据库连接
	initDatabases()
	
	// 启动HTTP服务器
	startServer()
}