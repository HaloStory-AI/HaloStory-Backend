package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName   string
	AppURL    string
	AppDebug  bool
	JWTSecret string
	JWTTTL    time.Duration
	TimeZone  string

	DBHost         string
	DBPort         string
	DBDatabase     string
	DBUsername     string
	DBPassword     string
	DBMaxOpenConns int
	DBMaxIdleConns int
	DBMaxLifetime  time.Duration

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	MailHost     string
	MailPort     int
	MailUsername string
	MailPassword string
	MailFrom     string
	MailFromName string

	S3Key      string
	S3Secret   string
	S3Region   string
	S3Bucket   string
	S3Endpoint string

	DiscordWebhookURL         string
	DiscordChildSafetyWebhook string

	TelegramBotToken string
	TelegramUserID   string

	AISystemKeys     map[string]string
	AIPriorityModels []string
	AIChainTimeout   time.Duration
	AIRequestTimeout time.Duration
	AIStreamTimeout  time.Duration
	AIMaxTokens      int

	StaticToken string
	CallVideos  []string
}

func Load() *Config {
	godotenv.Load()

	cfg := &Config{
		AppName:   getEnv("APP_NAME", "HaloStory"),
		AppURL:    getEnv("APP_URL", "http://localhost:18000"),
		AppDebug:  getEnvBool("APP_DEBUG", false),
		JWTSecret: getEnv("JWT_SECRET", "change-me"),
		JWTTTL:    getEnvDuration("JWT_TTL_HOURS", 168*time.Hour),
		TimeZone:  getEnv("TIMEZONE", "Asia/Jakarta"),

		DBHost:         getEnv("DB_HOST", "127.0.0.1"),
		DBPort:         getEnv("DB_PORT", "3306"),
		DBDatabase:     getEnv("DB_DATABASE", "bluetalk"),
		DBUsername:     getEnv("DB_USERNAME", "root"),
		DBPassword:     getEnv("DB_PASSWORD", ""),
		DBMaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 10),
		DBMaxLifetime:  getEnvDuration("DB_MAX_LIFETIME", 5*time.Minute),

		RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		MailHost:     getEnv("MAIL_HOST", ""),
		MailPort:     getEnvInt("MAIL_PORT", 587),
		MailUsername: getEnv("MAIL_USERNAME", ""),
		MailPassword: getEnv("MAIL_PASSWORD", ""),
		MailFrom:     getEnv("MAIL_FROM_ADDRESS", "noreply@halostory.app"),
		MailFromName: getEnv("MAIL_FROM_NAME", "HaloStory"),

		S3Key:      getEnv("S3_KEY", ""),
		S3Secret:   getEnv("S3_SECRET", ""),
		S3Region:   getEnv("S3_REGION", "us-east-1"),
		S3Bucket:   getEnv("S3_BUCKET", "bluetalk"),
		S3Endpoint: getEnv("S3_ENDPOINT", "https://rustfs.aneent.dev"),

		DiscordWebhookURL:         getEnv("DISCORD_WEBHOOK_URL", ""),
		DiscordChildSafetyWebhook: getEnv("DISCORD_CHILD_SAFETY_WEBHOOK", ""),

		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramUserID:   getEnv("TELEGRAM_USER_ID", ""),

		AISystemKeys: map[string]string{
			"gemini":            getEnv("AI_GEMINI_KEYS", ""),
			"gemini_priority":   getEnv("AI_GEMINI_PRIORITY_KEYS", ""),
			"groq":              getEnv("AI_GROQ_KEYS", ""),
			"openrouter":        getEnv("AI_OPENROUTER_KEYS", ""),
			"mistral":           getEnv("AI_MISTRAL_KEYS", ""),
			"cerebras":          getEnv("AI_CEREBRAS_KEYS", ""),
			"cohere":            getEnv("AI_COHERE_KEYS", ""),
			"huashang":          getEnv("AI_HUASHANG_KEYS", ""),
			"huashang_priority": getEnv("AI_HUASHANG_PRIORITY_KEYS", ""),
		},
		AIPriorityModels: splitEnv(getEnv("AI_PRIORITY_MODELS", "gemini-2.5-flash,gemini-2.5-flash-lite,gemini-2.0-flash")),
		AIChainTimeout:   getEnvDuration("AI_CHAIN_TIMEOUT_SECONDS", 60*time.Second),
		AIRequestTimeout: getEnvDuration("AI_REQUEST_TIMEOUT", 50*time.Second),
		AIStreamTimeout:  getEnvDuration("AI_STREAM_TIMEOUT_SECONDS", 40*time.Second),
		AIMaxTokens:      getEnvInt("AI_MAX_TOKENS", 800),

		StaticToken: getEnv("STATIC_AUTH_TOKEN", ""),
		CallVideos:  splitEnv(getEnv("CALL_VIDEOS", "https://archive.org/download/RickAstleyNeverGonnaGiveYouUp/RickAstleyNeverGonnaGiveYouUp.mp4")),
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func splitEnv(v string) []string {
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
