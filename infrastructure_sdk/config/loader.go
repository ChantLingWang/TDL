package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfig reads a YAML file from the given path and unmarshals it into the target struct.
func LoadConfig(path string, target interface{}) error {
	// 1. Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("config file not found at path: %s", path)
	}

	// 2. Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// 3. Unmarshal
	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}
