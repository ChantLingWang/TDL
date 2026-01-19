package templates

import "time"

// Template 模板 - 功能名列表
// 功能名列表和步骤数据的长度必须相同且对应关系正确
type EventTemplate struct {
	ID          string     `json:"id"`          // 模板ID
	Name        string     `json:"name"`        // 模板名称
	Topic       string     `json:"topic"`       // 事件功能名
	Description string     `json:"description"` // 模板描述
	Steps       []StepData `json:"steps"`       // 步骤列表，包含详细配置
	Enabled     bool       `json:"enabled"`     // 是否启用
	CreatedAt   time.Time  `json:"created_at"`  // 创建时间
}

// StepData 步骤数据
type StepData struct {
	Topic            string                 `json:"topic"`             // 步骤功能名
	Name             string                 `json:"name"`              // 步骤名称（可读性）
	CompensateAction string                 `json:"compensate_action"` // 补偿操作名
	MaxRetries       int                    `json:"max_retries"`       // 最大重试次数
	TimeoutMS        int                    `json:"timeout_ms"`        // 超时时间(ms)
	Data             map[string]interface{} `json:"data"`              // 步骤数据
}
