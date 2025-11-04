package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"user_service/app/database/pgsql"
	"user_service/app/database/mongodb"
	v1 "user_service/app/api/v1"
)

// initPostgreSQL 初始化PostgreSQL数据库连接
func initPostgreSQL() {
	dbManager := pgsql.GetDBManager()

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

// createApp 创建应用实例，类似于Python中的create_app
func createApp() *gin.Engine {
	// 初始化Gin引擎
	engine := gin.Default()
	
	// 获取用户路由实例
	userRouter := v1.NewRouter()
	userRouter.Engine = engine
	
	// 设置用户API路由
	userRouter.SetupRoutes()
	
	// 根路径
	engine.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "欢迎使用用户服务",
			"version": "v1.0.0",
			"health": "/api/v1/health",
		})
	})
	
	return engine
}

// initDatabases 初始化所有数据库连接
func initDatabases() {
	// 初始化PostgreSQL数据库连接
	initPostgreSQL()
	// 初始化MongoDB数据库连接
	initMongoDB()
}

// startServer 启动HTTP服务器
func startServer() {
	// 创建应用实例
	app := createApp()

	// 启动HTTP服务器
	log.Println("Starting server on :8080")
	if err := app.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func main() {
	// 初始化数据库连接
	initDatabases()
	
	// 启动HTTP服务器
	startServer()
}