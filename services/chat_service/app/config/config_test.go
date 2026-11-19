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
