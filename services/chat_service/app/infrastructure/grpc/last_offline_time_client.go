package grpc

import (
	"context"
	"log"
	"sync"

	"infrastructure_sdk/grpc/last_offline_time_grpc/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// LastOfflineTimeServiceAddr auth_service 的 gRPC 地址
	LastOfflineTimeServiceAddr = "localhost:50052"
)

// LastOfflineTimeClient gRPC 客户端
type LastOfflineTimeClient struct {
	conn   *grpc.ClientConn
	client proto.LastOfflineTimeServiceClient
}

// 全局客户端实例
var (
	lastOfflineTimeClientInstance *LastOfflineTimeClient
	lastOfflineTimeClientOnce    sync.Once
)

// GetLastOfflineTimeClient 获取 LastOfflineTimeClient 单例
func GetLastOfflineTimeClient() *LastOfflineTimeClient {
	lastOfflineTimeClientOnce.Do(func() {
		lastOfflineTimeClientInstance = NewLastOfflineTimeClient(LastOfflineTimeServiceAddr)
	})
	return lastOfflineTimeClientInstance
}

// NewLastOfflineTimeClient 创建新的 LastOfflineTimeClient
func NewLastOfflineTimeClient(addr string) *LastOfflineTimeClient {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Printf("Failed to create grpc client: %v", err)
		return nil
	}

	// 尝试连接
	conn.Connect()

	return &LastOfflineTimeClient{
		conn:   conn,
		client: proto.NewLastOfflineTimeServiceClient(conn),
	}
}

// UpdateLastOfflineTime 更新用户最后离线时间
func (c *LastOfflineTimeClient) UpdateLastOfflineTime(ctx context.Context, userID string) (*proto.UpdateLastOfflineTimeResponse, error) {
	if c.client == nil {
		return &proto.UpdateLastOfflineTimeResponse{
			Success: false,
		}, nil
	}

	req := &proto.UpdateLastOfflineTimeRequest{
		UserId: userID,
	}

	resp, err := c.client.UpdateLastOfflineTime(ctx, req)
	if err != nil {
		log.Printf("UpdateLastOfflineTime error: %v", err)
		return &proto.UpdateLastOfflineTimeResponse{
			Success: false,
		}, err
	}

	return resp, nil
}

// GetLastOfflineTime 获取用户最后离线时间
func (c *LastOfflineTimeClient) GetLastOfflineTime(ctx context.Context, userID string) (*proto.GetLastOfflineTimeResponse, error) {
	if c.client == nil {
		return &proto.GetLastOfflineTimeResponse{
			LastOfflineTime: 0,
		}, nil
	}

	req := &proto.GetLastOfflineTimeRequest{
		UserId: userID,
	}

	resp, err := c.client.GetLastOfflineTime(ctx, req)
	if err != nil {
		log.Printf("GetLastOfflineTime error: %v", err)
		return &proto.GetLastOfflineTimeResponse{
			LastOfflineTime: 0,
		}, err
	}

	return resp, nil
}

// Close 关闭连接
func (c *LastOfflineTimeClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
