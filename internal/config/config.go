package config

import "os"

// Config holds all application configuration.
type Config struct {
	TemporalHostPort   string
	WebhookPort        string
	GitHubWebhookSecret string
	LogBucket          string
	AWSRegion          string
}

// LoadConfig reads configuration from environment variables.
func LoadConfig() Config {
	return Config{
		TemporalHostPort:   getEnv("TEMPORAL_HOST_PORT", "localhost:7233"),
		WebhookPort:        getEnv("PORT", "8080"),
		GitHubWebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
		LogBucket:          os.Getenv("LOG_BUCKET"),
		AWSRegion:          getEnv("AWS_REGION", "us-east-1"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
