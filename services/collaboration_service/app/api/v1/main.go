package main

import (
	"log"
	"net/http"
	"os"
	"github.com/gin-gonic/gin"
)

func main() {
	// 创建Gin路由
	r := gin.Default()

	// 基本路由
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Collaboration Service is running"})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// 启动服务
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	log.Printf("Collaboration Service started on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
