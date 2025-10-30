package main

import (
	"log"

	"github.com/gin-gonic/gin"
	v1 "chant/user_service/app/api/v1"
	"chant/user_service/app/database"
)

func main() {
	// 初始化数据库连接
	dbManager := database.GetDBManager()
	if err := dbManager.Connect(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbManager.Close()

	// 初始化数据库表结构
	if err := dbManager.Initialize(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Println("Database initialized successfully")

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
