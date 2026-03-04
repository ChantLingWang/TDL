package templates

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// 模板路径全局变量
var TemplatePath = "templates/example_yaml"

// SetTemplatePath 设置模板路径
func SetTemplatePath(path string) {
	TemplatePath = path
}

// YAMLTemplate YAML模板结构
type YAMLTemplate struct {
	Name        string         `yaml:"name"`
	Topic       string         `yaml:"topic"`
	Description string         `yaml:"description"`
	Steps       []YAMLStepData `yaml:"steps"`
	Enabled     bool           `yaml:"enabled"`
}

// YAMLStepData YAML步骤数据结构
type YAMLStepData struct {
	Topic            string                 `yaml:"topic"`
	Name             string                 `yaml:"name"`
	CompensateAction string                 `yaml:"compensate_action"`
	MaxRetries       int                    `yaml:"max_retries"`
	TimeoutMS        int                    `yaml:"timeout_ms"`
	Data             map[string]interface{} `yaml:"data"`
}

// LoadTemplateFromYAML 通用函数：从YAML文件加载模板
func LoadTemplateFromYAML(filename string) (EventTemplate, error) {
	// 读取YAML文件
	yamlData, err := os.ReadFile(filename)
	if err != nil {
		return EventTemplate{}, fmt.Errorf("failed to read YAML file: %v", err)
	}

	// 解析YAML数据
	var yamlTemplate YAMLTemplate
	err = yaml.Unmarshal(yamlData, &yamlTemplate)
	if err != nil {
		return EventTemplate{}, fmt.Errorf("failed to parse YAML file: %v", err)
	}

	// 转换为EventTemplate格式
	var steps []StepData
	for _, yamlStep := range yamlTemplate.Steps {
		steps = append(steps, StepData(yamlStep))
	}

	// 确定模板ID：优先使用模板名称，如果为空则使用Topic
	templateID := yamlTemplate.Name

	template := EventTemplate{
		ID:          templateID,
		Name:        yamlTemplate.Name,
		Topic:       yamlTemplate.Topic,
		Description: yamlTemplate.Description,
		Steps:       steps,
		Enabled:     yamlTemplate.Enabled,
		CreatedAt:   time.Now(),
	}

	return template, nil
}

// GetTemplatePath 获取模板文件的完整路径
func GetTemplatePath(templateName string) string {
	return fmt.Sprintf("%s/%s.yaml", TemplatePath, templateName)
}

// LoadTemplate 加载指定的模板文件
func LoadTemplate(templateName string) (EventTemplate, error) {
	return LoadTemplateFromYAML(GetTemplatePath(templateName))
}
