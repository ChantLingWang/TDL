package grpc

import (
	"context"
	"log"
	"sync"

	"chat_service/app/infrastructure/grpc/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// AuthServiceAddr auth_service 的 gRPC 地址
	AuthServiceAddr = "localhost:50051"
)

// AuthClient gRPC 认证客户端
type AuthClient struct {
	conn   *grpc.ClientConn
	client proto.AuthServiceClient
}

// 全局客户端实例
var (
	authClientInstance *AuthClient
	authClientOnce     sync.Once
)

// GetAuthClient 获取 AuthClient 单例
func GetAuthClient() *AuthClient {
	authClientOnce.Do(func() {
		authClientInstance = NewAuthClient(AuthServiceAddr)
	})
	return authClientInstance
}

// NewAuthClient 创建新的 AuthClient
func NewAuthClient(addr string) *AuthClient {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Printf("Failed to create grpc client: %v", err)
		return nil
	}

	// 尝试连接
	conn.Connect()

	return &AuthClient{
		conn:   conn,
		client: proto.NewAuthServiceClient(conn),
	}
}

// VerifyToken 验证 Token 并返回用户信息
func (c *AuthClient) VerifyToken(ctx context.Context, token string) (*proto.VerifyTokenResponse, error) {
	if c.client == nil {
		return &proto.VerifyTokenResponse{
			Valid:   false,
			Message: "auth client not initialized",
		}, nil
	}

	req := &proto.VerifyTokenRequest{
		Token: token,
	}

	resp, err := c.client.VerifyToken(ctx, req)
	if err != nil {
		log.Printf("VerifyToken error: %v", err)
		return &proto.VerifyTokenResponse{
			Valid:   false,
			Message: err.Error(),
		}, err
	}

	return resp, nil
}

// Close 关闭连接
func (c *AuthClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
