package mongodb

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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

	// 幂等去重：message_id 已存在则跳过（Kafka 重试/重启回放保护）
	dupFilter := bson.M{
		"session_id":            sessionID,
		"date_identifier":       getPrivateMessageHistoryDocTime(),
		"messages.message_id":   message.MessageID,
	}
	if count, err := collection.CountDocuments(context.Background(), dupFilter); err == nil && count > 0 {
		return nil
	}

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

// GetHistoryMessages 获取私聊历史消息（分发函数）
func (service *PrivateMessageHistoryService) GetHistoryMessages(sessionID string, cursor, startTime, endTime int64, keyword string, limit int) ([]Message, error) {
	hasCursor := cursor > 0

	if hasCursor {
		return service.GetHistoryMessagesByCursor(sessionID, cursor, keyword, limit)
	}
	return service.GetHistoryMessagesByTimeRange(sessionID, startTime, endTime, keyword, limit)
}

// GetHistoryMessagesByCursor 按游标获取私聊历史消息
func (service *PrivateMessageHistoryService) GetHistoryMessagesByCursor(sessionID string, cursor int64, keyword string, limit int) ([]Message, error) {
	if cursor <= 0 {
		return nil, fmt.Errorf("invalid cursor: must be greater than 0")
	}

	db := GetMongoDBManager().GetDatabase()
	if db == nil {
		return nil, fmt.Errorf("failed to get database instance")
	}

	searchCursor := time.Unix(cursor, 0)
	var allMessages []Message

	now := time.Now()
	for i := 0; i < 30; i++ {
		collectionTime := now.AddDate(0, 0, -i)
		collectionName := "private_message_history_" + collectionTime.Format("200601")
		collection := db.Collection(collectionName)

		filter := bson.M{
			"session_id": sessionID,
		}

		timeCondition := bson.M{"$lt": searchCursor}

		if keyword != "" {
			filter["messages"] = bson.M{
				"$elemMatch": bson.M{
					"timestamp": timeCondition,
					"content":   bson.M{"$regex": keyword, "$options": "i"},
				},
			}
		} else {
			filter["messages"] = bson.M{
				"$elemMatch": bson.M{
					"timestamp": timeCondition,
				},
			}
		}

		opts := options.Find().
			SetSort(bson.M{"end_time": -1}).
			SetLimit(int64(limit))

		cursor, err := collection.Find(context.Background(), filter, opts)
		if err != nil {
			continue
		}

		for cursor.Next(context.Background()) {
			var doc PrivateMessageHistory
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			for _, msg := range doc.Messages {
				if !msg.Timestamp.Before(searchCursor) {
					continue
				}
				if keyword != "" {
					matched, err := regexp.MatchString(`(?i)`+keyword, msg.Content)
					if err != nil || !matched {
						continue
					}
				}
				allMessages = append(allMessages, msg)
			}
		}
		cursor.Close(context.Background())

		if len(allMessages) >= limit {
			break
		}
	}

	sort.Slice(allMessages, func(i, j int) bool {
		return allMessages[i].Timestamp.After(allMessages[j].Timestamp)
	})

	if len(allMessages) > limit {
		allMessages = allMessages[:limit]
	}

	return allMessages, nil
}

// GetHistoryMessagesByTimeRange 按时间范围获取私聊历史消息
func (service *PrivateMessageHistoryService) GetHistoryMessagesByTimeRange(sessionID string, startTime, endTime int64, keyword string, limit int) ([]Message, error) {
	db := GetMongoDBManager().GetDatabase()
	if db == nil {
		return nil, fmt.Errorf("failed to get database instance")
	}

	var searchStartTime, searchEndTime time.Time
	if startTime > 0 {
		searchStartTime = time.Unix(startTime, 0)
	}
	if endTime > 0 {
		searchEndTime = time.Unix(endTime, 0)
	}

	var allMessages []Message
	now := time.Now()

	// 时间范围查询的三种情况：
	// 1. 只有 startTime：按 startTime 往后（更晚时间）查30条
	// 2. 只有 endTime：按 endTime 往前（更早时间）查30条
	// 3. 两个都有：从 startTime 开始往后查，直到 endTime
	hasStartTime := startTime > 0
	hasEndTime := endTime > 0

	for i := 0; i < 30; i++ {
		collectionTime := now.AddDate(0, 0, -i)
		collectionName := "private_message_history_" + collectionTime.Format("200601")
		collection := db.Collection(collectionName)

		// 构建基础过滤条件
		filter := bson.M{
			"session_id": sessionID,
		}

		// 构建时间条件
		var timeCondition bson.M
		if hasStartTime && hasEndTime {
			// 两个时间都有：查询 startTime <= 消息 <= endTime
			timeCondition = bson.M{
				"$gte": searchStartTime,
				"$lte": searchEndTime,
			}
		} else if hasStartTime {
			// 只有 startTime：按 startTime 往后（更晚时间）查
			timeCondition = bson.M{
				"$gte": searchStartTime,
			}
		} else if hasEndTime {
			// 只有 endTime：按 endTime 往前（更早时间）查
			timeCondition = bson.M{
				"$lte": searchEndTime,
			}
		}

		// 构建消息过滤条件
		if len(timeCondition) > 0 {
			if keyword != "" {
				filter["messages"] = bson.M{
					"$elemMatch": bson.M{
						"timestamp": timeCondition,
						"content":   bson.M{"$regex": keyword, "$options": "i"},
					},
				}
			} else {
				filter["messages"] = bson.M{
					"$elemMatch": bson.M{
						"timestamp": timeCondition,
					},
				}
			}
		} else if keyword != "" {
			filter["messages"] = bson.M{
				"$elemMatch": bson.M{
					"content": bson.M{"$regex": keyword, "$options": "i"},
				},
			}
		}

		// 设置排序和limit
		// 只有 startTime 时，需要按时间正序（asc），从早到晚
		// 只有 endTime 时，需要按时间倒序（desc），从晚到早
		// 两个都有时，按时间倒序
		var opts *options.FindOptions
		if hasStartTime && !hasEndTime {
			// 只有 startTime，按时间正序，从早到晚
			opts = options.Find().
				SetSort(bson.M{"end_time": 1}).
				SetLimit(int64(limit))
		} else {
			// 其他情况按时间倒序，最新的在前
			opts = options.Find().
				SetSort(bson.M{"end_time": -1}).
				SetLimit(int64(limit))
		}

		cursor, err := collection.Find(context.Background(), filter, opts)
		if err != nil {
			continue
		}

		for cursor.Next(context.Background()) {
			var doc PrivateMessageHistory
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			// 过滤消息
			for _, msg := range doc.Messages {
				// 时间范围过滤
				if hasStartTime && msg.Timestamp.Before(searchStartTime) {
					continue
				}
				if hasEndTime && msg.Timestamp.After(searchEndTime) {
					continue
				}
				// 关键字过滤
				if keyword != "" {
					matched, err := regexp.MatchString(`(?i)`+keyword, msg.Content)
					if err != nil || !matched {
						continue
					}
				}
				allMessages = append(allMessages, msg)
			}
		}
		cursor.Close(context.Background())

		// 如果已经收集够消息了，就停止
		if len(allMessages) >= limit {
			break
		}
	}

	// 按时间倒序排列（最新的在前）
	sort.Slice(allMessages, func(i, j int) bool {
		return allMessages[i].Timestamp.After(allMessages[j].Timestamp)
	})

	// 取前 N 条
	if len(allMessages) > limit {
		allMessages = allMessages[:limit]
	}

	return allMessages, nil
}
