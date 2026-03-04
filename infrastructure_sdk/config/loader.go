package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfig reads a YAML file from the given path and unmarshals it into the target struct.
func LoadConfig(path string, target interface{}) error {
	// 1. 检查配置文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("config file not found at path: %s", path)
	}

	// 2. 读取配置文件内容
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// 3. 解析配置文件内容
	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}
