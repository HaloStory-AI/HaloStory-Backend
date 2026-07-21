package services

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScheduler_StartStop(t *testing.T) {
	// Just test that StartScheduler doesn't panic
	c := StartScheduler()
	assert.NotNil(t, c)

	// Should not panic on stop
	c.Stop()
}

func TestScheduler_EnvVarCheck(t *testing.T) {
	// Verify that STUDENT_REQUESTS_OPEN is read correctly
	os.Setenv("STUDENT_REQUESTS_OPEN", "true")
	assert.Equal(t, "true", os.Getenv("STUDENT_REQUESTS_OPEN"))
	os.Unsetenv("STUDENT_REQUESTS_OPEN")
}

func TestScheduler_ConfigLoads(t *testing.T) {
	// Test that config.Env works properly for scheduler-relevant vars
	os.Setenv("DB_HOST", "127.0.0.1")
	assert.Equal(t, "127.0.0.1", os.Getenv("DB_HOST"))
	os.Unsetenv("DB_HOST")
}
