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

const (
	MaxMessagesPerBucket = 500 // 每个分桶最大消息数
)

type Message struct {
	Timestamp   time.Time `bson:"timestamp"`    // 时间戳
	Content     string    `bson:"content"`      // 消息内容
	TouserID    string    `bson:"touser_id"`    // 目标用户ID
	MessageID   string    `bson:"message_id"`   // 消息ID
	MessageType string    `bson:"message_type"` // 消息类型
	IsActive    bool      `bson:"is_active"`    // 是否可见
}

// GroupMessageHistory 表示组群聊天记录的结构
type GroupMessageHistory struct {
	GroupID        string    `bson:"group_id"`        // 群组ID
	DateIdentifier string    `bson:"date_identifier"` // 日期标识符
	Messages       []Message `bson:"messages"`        // 消息数组
	Count          int       `bson:"count"`           // 当前文档消息数量，用于分桶控制
	StartTime      time.Time `bson:"start_time"`      // 桶内第一条消息时间
	EndTime        time.Time `bson:"end_time"`        // 桶内最后一条消息时间
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

	// 构造查找条件：增加消息数量限制，实现自动分桶
	filter := bson.M{
		"group_id":        groupID,
		"date_identifier": getGroupMessageHistoryDocTime(),
		"count":           bson.M{"$lt": MaxMessagesPerBucket}, // 每个桶最大消息数
	}

	// 构建更新条件
	update := bson.M{
		"$setOnInsert": bson.M{
			"group_id":        groupID,
			"date_identifier": getGroupMessageHistoryDocTime(),
			"start_time":      message.Timestamp, // 新建桶时设置起始时间
		},
		"$push": bson.M{
			"messages": *message,
		},
		"$inc": bson.M{
			"count": 1, // 消息数量加 1
		},
		"$set": bson.M{
			"end_time": message.Timestamp, // 每次都更新结束时间
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

// GetGroupMessagesByDate 获取指定日期群组的所有消息（自动合并所有分桶）
func (service *GroupMessageHistoryService) GetGroupMessagesByDate(groupID string, date string) ([]Message, error) {
	db := GetMongoDBManager().GetDatabase()
	if db == nil {
		return nil, fmt.Errorf("failed to get database instance")
	}

	// 这里的 collection name 生成逻辑可能需要根据 date 参数调整
	// 假设 date 格式为 "20060102"，则 collection 后缀为 "200601"
	if len(date) < 6 {
		return nil, fmt.Errorf("invalid date format")
	}
	collectionName := "group_message_history_" + date[:6]
	collection := db.Collection(collectionName)

	filter := bson.M{
		"group_id":        groupID,
		"date_identifier": date,
	}

	// 按 _id 正序排序，确保桶的顺序正确
	opts := options.Find().SetSort(bson.M{"_id": 1})

	cursor, err := collection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find group messages: %w", err)
	}
	defer cursor.Close(context.Background())

	var allMessages []Message
	for cursor.Next(context.Background()) {
		var doc GroupMessageHistory
		if err := cursor.Decode(&doc); err != nil {
			log.Printf("Failed to decode group message history doc: %v", err)
			continue
		}
		allMessages = append(allMessages, doc.Messages...)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return allMessages, nil
}
