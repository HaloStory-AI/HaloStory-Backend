package tests

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Setenv("APP_KEY", "test_secret_key_for_testing")
	os.Setenv("APP_DEBUG", "false")

	code := m.Run()
	os.Exit(code)
}
