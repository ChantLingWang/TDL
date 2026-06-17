//go:build integration

// 双实例端到端集成测试
//
// 前置条件：docker-compose up -d 启动基础设施
// 运行：go run -tags=integration ./integration_test/
//
// 流程：
//  1. 编译 chat_service 二进制
//  2. 启动实例1 (port=18083, chat_group_id=int-test-1)
//  3. 启动实例2 (port=18084, chat_group_id=int-test-2)
//  4. 生成两个 JWT token，客户端 A 连实例1，客户端 B 连实例2
//  5. A 在实例1 发一条消息 → 通过 Kafka 扇出 → 实例2 推送给 B

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// JWT --------------------------------
func deriveSecretKey() string {
	h := sha256.Sum256([]byte("LianLingHao_1001_liberation"))
	return hex.EncodeToString(h[:])
}

func base64urlEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func generateJWT(userID, username, email string) string {
	secret := deriveSecretKey()
	now := time.Now()

	header := `{"alg":"HS256","typ":"JWT"}`
	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"user_id":  userID,
		"username": username,
		"email":    email,
		"exp":      now.Add(1 * time.Hour).Unix(),
		"iat":      now.Unix(),
		"jti":      "integration-test",
		"type":     "access",
	})

	msg := base64urlEncode([]byte(header)) + "." + base64urlEncode(payloadBytes)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	sig := base64urlEncode(mac.Sum(nil))
	return msg + "." + sig
}

// TCP / HTTP helpers ----------------
func tcpDial(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func dialWS(url, token string) (*websocket.Conn, error) {
	header := http.Header{"Authorization": {"Bearer " + token}}
	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	return conn, err
}

func restPost(url string, body interface{}, token string) (*http.Response, error) {
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return http.DefaultClient.Do(req)
}

// 实例管理 ---------------------------
type instance struct {
	cmd    *exec.Cmd
	port   string
	closed chan struct{}
}

func startInstance(binary, port, chatGroupID, cwd string) (*instance, error) {
	cmd := exec.Command(binary)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(),
		"SERVER_PORT="+port,
		"CHAT_GROUP_ID="+chatGroupID,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	closed := make(chan struct{})
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		cmd.Wait()
		close(closed)
	}()

	return &instance{cmd: cmd, port: port, closed: closed}, nil
}

func (i *instance) waitReady(timeout time.Duration) bool {
	deadline := time.After(timeout)
	addr := "localhost:" + i.port
	for {
		select {
		case <-deadline:
			return false
		case <-i.closed:
			return false
		default:
			if tcpDial(addr) {
				return true
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func (i *instance) kill() { i.cmd.Process.Kill() }

// main --------------------------------
func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	// 检查基础设施
	for _, addr := range []string{"localhost:9094", "localhost:5432", "localhost:27017", "localhost:6379", "localhost:50051"} {
		if !tcpDial(addr) {
			log.Fatalf("❌ %s unreachable — run docker-compose up -d first", addr)
		}
	}
	log.Println("✅ infrastructure reachable")

	// 编译
	cwd := "."
	binary := "/tmp/chat-service-integration"
	buildCmd := exec.Command("go", "build", "-o", binary, ".")
	buildCmd.Dir = cwd
	if out, err := buildCmd.CombinedOutput(); err != nil {
		log.Fatalf("build failed: %v\n%s", err, out)
	}
	log.Println("✅ built chat-service binary")
	defer os.Remove(binary)

	// 启动两个实例
	i1, err := startInstance(binary, "18083", "int-test-1", cwd)
	if err != nil {
		log.Fatalf("start instance1: %v", err)
	}
	defer i1.kill()

	i2, err := startInstance(binary, "18084", "int-test-2", cwd)
	if err != nil {
		log.Fatalf("start instance2: %v", err)
	}
	defer i2.kill()

	if !i1.waitReady(15 * time.Second) {
		log.Fatal("❌ instance 1 did not become ready")
	}
	log.Println("✅ instance 1 ready on :18083")

	if !i2.waitReady(15 * time.Second) {
		log.Fatal("❌ instance 2 did not become ready")
	}
	log.Println("✅ instance 2 ready on :18084")

	// JWT
	tokenA := generateJWT("test-user-a", "TestA", "a@test.com")
	tokenB := generateJWT("test-user-b", "TestB", "b@test.com")

	// WebSocket 连接
	wsA, err := dialWS("ws://localhost:18083/api/v1/ws", tokenA)
	if err != nil {
		log.Fatalf("ws A connect: %v", err)
	}
	defer wsA.Close()
	log.Println("✅ client A connected to instance 1")

	wsB, err := dialWS("ws://localhost:18084/api/v1/ws", tokenB)
	if err != nil {
		log.Fatalf("ws B connect: %v", err)
	}
	defer wsB.Close()
	log.Println("✅ client B connected to instance 2")

	// 初始化测试用户
	for _, c := range []struct {
		baseURL string
		userID  string
		token   string
	}{
		{"http://localhost:18083", "test-user-a", tokenA},
		{"http://localhost:18084", "test-user-b", tokenB},
	} {
		resp, _ := restPost(fmt.Sprintf("%s/api/v1/users/%s/init?username=%s", c.baseURL, c.userID, c.baseURL), nil, c.token)
		if resp != nil && resp.StatusCode == http.StatusOK {
			log.Printf("✅ user %s initialized", c.userID)
		} else {
			log.Printf("⚠️ user %s init returned status %v (may already exist)", c.userID, resp)
		}
	}

	// 创建群组
	type groupResp struct {
		GroupID   string `json:"group_id"`
		GroupName string `json:"group_name"`
	}
	resp, _ := restPost("http://localhost:18083/api/v1/groups", map[string]string{
		"group_name": "integration-test-group",
		"creator_id": "test-user-a",
	}, tokenA)
	var gr groupResp
	if resp != nil {
		json.NewDecoder(resp.Body).Decode(&gr)
		resp.Body.Close()
	}
	if gr.GroupID == "" {
		log.Fatal("❌ failed to create group")
	}
	log.Printf("✅ group created: %s", gr.GroupID)

	// B 加入群组
	resp, _ = restPost("http://localhost:18084/api/v1/groups/join", map[string]string{
		"group_id": gr.GroupID,
		"user_id":  "test-user-b",
	}, tokenB)
	if resp != nil && resp.StatusCode == http.StatusOK {
		log.Println("✅ user B joined group")
	} else {
		log.Fatal("❌ user B failed to join group")
	}

	// 发送消息
	chatMsg := map[string]interface{}{
		"type": "chat",
		"content": map[string]interface{}{
			"conversation_type": "group",
			"sender_id":         "test-user-a",
			"group_id":          gr.GroupID,
			"text":              "hello from integration test @ " + time.Now().Format(time.RFC3339),
			"message_id":        "int-test-msg-" + time.Now().Format("150405"),
			"message_type":      "text",
		},
	}
	if err := wsA.WriteJSON(chatMsg); err != nil {
		log.Fatalf("❌ send message: %v", err)
	}
	log.Println("✅ message sent from client A (instance 1)")

	// 读回
	wsB.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, raw, err := wsB.ReadMessage()
	if err != nil {
		log.Fatalf("❌ client B read: %v", err)
	}

	var received map[string]interface{}
	json.Unmarshal(raw, &received)
	log.Printf("📩 client B received: type=%v sender=%v", received["type"], received["sender"])

	if received["type"] != nil {
		log.Println("✅✅✅ INTEGRATION TEST PASSED — cross-instance message delivery works!")
	} else {
		log.Fatalf("❌ INTEGRATION TEST FAILED — unexpected message: %s", string(raw))
	}
}
