package mongodb

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/bson"
)


type Message struct {
	Timestamp   time.Time `bson:"timestamp"`    // 时间戳
	Content     string    `bson:"content"`      // 消息内容
	UserID      string    `bson:"user_id"`      // 用户ID
	Username    string    `bson:"username"`     // 用户名
	MessageID   string    `bson:"message_id"`   // 消息ID
	MessageType string    `bson:"message_type"` // 消息类型
	IsActive    bool      `bson:"is_active"`    // 是否可见
}


// GroupMessageHistory 表示组群聊天记录的结构
type GroupMessageHistory struct {
	GroupID        string    `bson:"group_id"`        // 群组ID
	DateIdentifier string    `bson:"date_identifier"` // 日期标识符
	Messages       []Message `bson:"messages"`        // 消息数组
}


// GroupMessageHistoryService 组群聊天记录服务
type GroupMessageHistoryService struct {
	client *mongo.Client
}


// 定义单例模式确保只有一个组群聊天记录服务实例
var (
	groupMsgHistoryInstance *GroupMessageHistoryService
	groupMsgHistoryOnce     sync.Once
)


// GetGroupMessageHistoryService 获取组群聊天记录服务实例
func GetGroupMessageHistoryService() *GroupMessageHistoryService {
	groupMsgHistoryOnce.Do(func() {
		groupMsgHistoryInstance = &GroupMessageHistoryService{
			client: GetMongoDBManager().client,
		}
	})
	return groupMsgHistoryInstance
}


// getGroupMessageHistoryCollectionTime 获取组群聊天记录集合时间
func getGroupMessageHistoryCollectionTime() string {
	now := time.Now()
	// Go语言的时间格式化使用特殊的模板系统：
	// "2006"代表年份，"01"代表月份
	// 例如：2025年11月会格式化为"202511"
	return now.Format("200601")
}


// getGroupMessageHistoryDocTime 获取组群聊天记录文档时间
func getGroupMessageHistoryDocTime() string {
	now := time.Now()
	return now.Format("20060102")
}


// 向消息数组中追加新消息
func (service *GroupMessageHistoryService) AddGroupMessageByUser(groupID string, message *Message) error {
	// 获取数据库实例
	db := GetMongoDBManager().GetDatabase()
	if db == nil {
		log.Printf("Failed to get database instance for group %s", groupID)
		return fmt.Errorf("failed to get database instance")
	}

	// 获取集合
	collection := db.Collection("group_message_history_" + getGroupMessageHistoryCollectionTime())

	// 构造查找条件
	filter := bson.M{
		"group_id": groupID,
		"date_identifier": getGroupMessageHistoryDocTime(),
	}

	// 构建更新条件
	update := bson.M{
		"$setOnInsert": bson.M{
			"group_id":   groupID,
			"date_identifier":  getGroupMessageHistoryDocTime(),
			"messages":   []Message{},
		},
		"$push": bson.M{
			"messages":    *message,
		},
	}

	// 使用Upsert操作，自动处理文档存在性检查
	_, err := collection.UpdateOne(context.Background(), filter, update, options.Update().SetUpsert(true))
	if err != nil {
		log.Printf("Failed to upsert group message for group %s: %v", groupID, err)
		return fmt.Errorf("failed to upsert group message: %w", err)
	}

	return nil
}
