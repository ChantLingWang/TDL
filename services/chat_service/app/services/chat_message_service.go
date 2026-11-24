package services

import (
	"strings"

	"chat_service/app/api/models"
)

// HandleChat 处理用户发往 AI 的消息：只做校验，然后交给 round 分配器。
// round 分配与 Kafka 投递都在同一个串行段里完成，保证同会话顺序。
func HandleChat(userInfo *UserInfo, content models.ChatMessageRequest) {
	if strings.TrimSpace(content.Text) == "" && len(content.Parts) == 0 {
		return
	}
	GetRoundAllocator().Submit(userInfo, content)
}
