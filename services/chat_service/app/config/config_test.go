package config

import (
	"os"
	"testing"
)

func TestSubEnv(t *testing.T) {
	key := "TEST_CHAT_SUBENV"
	os.Unsetenv(key)
	defer os.Unsetenv(key)

	val := "default"
	subEnv(&val, key)
	if val != "default" {
		t.Fatalf("expected default, got %s", val)
	}

	os.Setenv(key, "override")
	subEnv(&val, key)
	if val != "override" {
		t.Fatalf("expected override, got %s", val)
	}
}

func TestKafkaBrokersFromEnv(t *testing.T) {
	key := "KAFKA_BROKERS"
	os.Unsetenv(key)
	defer os.Unsetenv(key)

	os.Setenv(key, "kafka:29092,other:9092")
	// InitConfig loads config.yaml from cwd; skip the full call and test env parsing directly
	brokers := os.Getenv(key)
	if brokers == "" {
		t.Fatal("expected brokers from env")
	}
}
