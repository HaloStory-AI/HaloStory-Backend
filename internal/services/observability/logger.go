package observability

import (
	"log/slog"
	"math/rand"
	"os"

	"halostory_go_backend/internal/config"
)

var Logger *slog.Logger

func InitLogger() {
	level := slog.LevelInfo
	if config.Env("APP_DEBUG", "false") == "true" {
		level = slog.LevelDebug
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})
	Logger = slog.New(handler)
	slog.SetDefault(Logger)
}

// ShouldSample returns true 10% of time or when APP_DEBUG=true
func ShouldSample() bool {
	if config.Env("APP_DEBUG", "false") == "true" {
		return true
	}
	return rand.Float64() < 0.1
}

func Info(msg string, args ...any) {
	if Logger != nil {
		Logger.Info(msg, args...)
	}
}

func Warn(msg string, args ...any) {
	if Logger != nil {
		Logger.Warn(msg, args...)
	}
}

func Error(msg string, args ...any) {
	if Logger != nil {
		Logger.Error(msg, args...)
	}
}

func Debug(msg string, args ...any) {
	if Logger != nil {
		Logger.Debug(msg, args...)
	}
}
