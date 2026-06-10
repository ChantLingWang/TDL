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

const (
	MaxMessagesPerBucket = 500 // 每个分桶最大消息数
)

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

	// 幂等去重：message_id 已存在则跳过（Kafka 重试/重启回放保护）
	dupFilter := bson.M{
		"group_id":              groupID,
		"date_identifier":       getGroupMessageHistoryDocTime(),
		"messages.message_id":   message.MessageID,
	}
	if count, err := collection.CountDocuments(context.Background(), dupFilter); err == nil && count > 0 {
		return nil
	}

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

// GetUnreadMessages 获取未读消息（根据会话ID列表和时间）
// conversationIDs: 会话ID列表
// afterTime: 时间戳（秒），只获取该时间之后的消息
// limit: 返回数量限制
func (service *GroupMessageHistoryService) GetUnreadMessages(conversationIDs []string, afterTime int64, limit int) ([]Message, int, error) {
	if len(conversationIDs) == 0 {
		return nil, 0, nil
	}

	db := GetMongoDBManager().GetDatabase()
	if db == nil {
		return nil, 0, fmt.Errorf("failed to get database instance")
	}

	afterTimestamp := time.Unix(afterTime, 0)
	var allMessages []Message
	var totalCount int

	// 获取过去几天的 collection
	now := time.Now()
	for i := 0; i < 3; i++ { // 最多查最近3天
		collectionTime := now.AddDate(0, 0, -i)
		collectionName := "group_message_history_" + collectionTime.Format("200601")
		collection := db.Collection(collectionName)

		// 查询条件：group_id 在会话列表中，且消息时间大于 afterTime
		filter := bson.M{
			"group_id": bson.M{"$in": conversationIDs},
			"messages": bson.M{
				"$elemMatch": bson.M{
					"timestamp": bson.M{"$gt": afterTimestamp},
				},
			},
		}

		// 查询消息
		opts := options.Find().
			SetSort(bson.M{"end_time": -1}). // 按结束时间倒序，最新的在前
			SetLimit(int64(limit))

		cursor, err := collection.Find(context.Background(), filter, opts)
		if err != nil {
			continue
		}

		for cursor.Next(context.Background()) {
			var doc GroupMessageHistory
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			// 过滤并统计时间大于 afterTime 的消息
			for _, msg := range doc.Messages {
				if msg.Timestamp.After(afterTimestamp) {
					allMessages = append(allMessages, msg)
					totalCount++ // 累加实际未读消息数量
				}
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

	return allMessages, totalCount, nil
}

// GetHistoryMessages 获取历史消息（支持时间范围和关键字搜索）
// conversationID: 会话ID（群ID）
// GetHistoryMessages 获取历史消息（分发函数）
func (service *GroupMessageHistoryService) GetHistoryMessages(conversationID string, cursor, startTime, endTime int64, keyword string, limit int) ([]Message, error) {
	hasCursor := cursor > 0

	if hasCursor {
		return service.GetHistoryMessagesByCursor(conversationID, cursor, keyword, limit)
	}
	return service.GetHistoryMessagesByTimeRange(conversationID, startTime, endTime, keyword, limit)
}

// GetHistoryMessagesByCursor 按游标获取历史消息
func (service *GroupMessageHistoryService) GetHistoryMessagesByCursor(conversationID string, cursor int64, keyword string, limit int) ([]Message, error) {
	if cursor <= 0 {
		return nil, fmt.Errorf("invalid cursor: must be greater than 0")
	}

	db := GetMongoDBManager().GetDatabase()
	if db == nil {
		return nil, fmt.Errorf("failed to get database instance")
	}

	// cursor 是 Unix 时间戳（秒），转换为 time.Time 用于时间比较
	searchCursor := time.Unix(cursor, 0)
	var allMessages []Message

	now := time.Now()
	for i := 0; i < 30; i++ {
		collectionTime := now.AddDate(0, 0, -i)
		collectionName := "group_message_history_" + collectionTime.Format("200601")
		collection := db.Collection(collectionName)

		filter := bson.M{
			"group_id": conversationID,
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
			var doc GroupMessageHistory
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

// GetHistoryMessagesByTimeRange 按时间范围获取历史消息
func (service *GroupMessageHistoryService) GetHistoryMessagesByTimeRange(conversationID string, startTime, endTime int64, keyword string, limit int) ([]Message, error) {
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
		collectionName := "group_message_history_" + collectionTime.Format("200601")
		collection := db.Collection(collectionName)

		// 构建基础过滤条件
		filter := bson.M{
			"group_id": conversationID,
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
			var doc GroupMessageHistory
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
