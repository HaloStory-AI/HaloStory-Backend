package config

import (
	"os"
	"testing"
)

func TestEnv_WithExistingKey(t *testing.T) {
	os.Setenv("TEST_KEY", "hello")
	defer os.Unsetenv("TEST_KEY")

	result := Env("TEST_KEY", "default")
	if result != "hello" {
		t.Errorf("expected 'hello', got '%s'", result)
	}
}

func TestEnv_WithFallback(t *testing.T) {
	os.Unsetenv("NONEXISTENT_KEY")
	result := Env("NONEXISTENT_KEY", "fallback")
	if result != "fallback" {
		t.Errorf("expected 'fallback', got '%s'", result)
	}
}
