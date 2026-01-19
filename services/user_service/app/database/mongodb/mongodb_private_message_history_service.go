package mongodb

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PrivateMessageHistory 表示私聊记录的结构
type PrivateMessageHistory struct {
	SessionID      string    `bson:"session_id"`      // 会话ID (两个用户ID排序后的组合，确保唯一性)
	DateIdentifier string    `bson:"date_identifier"` // 日期标识符
	Messages       []Message `bson:"messages"`        // 消息数组
}

// PrivateMessageHistoryService 私聊记录服务
type PrivateMessageHistoryService struct {
	client *mongo.Client
}

// 定义单例模式
var (
	privateMsgHistoryInstance *PrivateMessageHistoryService
	privateMsgHistoryOnce     sync.Once
)

// GetPrivateMessageHistoryService 获取私聊记录服务实例
func GetPrivateMessageHistoryService() *PrivateMessageHistoryService {
	privateMsgHistoryOnce.Do(func() {
		privateMsgHistoryInstance = &PrivateMessageHistoryService{
			client: GetMongoDBManager().client,
		}
	})
	return privateMsgHistoryInstance
}

// getPrivateMessageHistoryCollectionTime 获取集合时间
func getPrivateMessageHistoryCollectionTime() string {
	now := time.Now()
	return now.Format("200601")
}

// getPrivateMessageHistoryDocTime 获取文档时间
func getPrivateMessageHistoryDocTime() string {
	now := time.Now()
	return now.Format("20060102")
}

// GenerateSessionID 生成唯一的会话ID (UserID1 < UserID2)
func GenerateSessionID(userID1, userID2 string) string {
	if userID1 < userID2 {
		return userID1 + "_" + userID2
	}
	return userID2 + "_" + userID1
}

// AddPrivateMessage 保存私聊消息
func (service *PrivateMessageHistoryService) AddPrivateMessage(senderID, receiverID string, message *Message) error {
	db := GetMongoDBManager().GetDatabase()
	if db == nil {
		return fmt.Errorf("failed to get database instance")
	}

	collection := db.Collection("private_message_history_" + getPrivateMessageHistoryCollectionTime())

	sessionID := GenerateSessionID(senderID, receiverID)

	filter := bson.M{
		"session_id":      sessionID,
		"date_identifier": getPrivateMessageHistoryDocTime(),
	}

	update := bson.M{
		"$setOnInsert": bson.M{
			"session_id":      sessionID,
			"date_identifier": getPrivateMessageHistoryDocTime(),
			// "messages":        []Message{}, // 移除这行，避免与 $push 冲突
		},
		"$push": bson.M{
			"messages": *message,
		},
	}

	_, err := collection.UpdateOne(context.Background(), filter, update, options.Update().SetUpsert(true))
	if err != nil {
		log.Printf("Failed to upsert private message for session %s: %v", sessionID, err)
		return fmt.Errorf("failed to upsert private message: %w", err)
	}

	return nil
}
