package config

import (
	"os"
	"strings"
)

// Config holds all application configuration.
type Config struct {
	TemporalHostPort    string
	TemporalWebURL      string
	DashboardURL        string
	WebhookPort         string
	GitHubWebhookSecret string
	GitHubToken         string
	LogBucket           string
	AWSRegion           string
}

// LoadConfig reads configuration from environment variables.
func LoadConfig() Config {
	return Config{
		TemporalHostPort:    getEnv("TEMPORAL_HOST_PORT", "localhost:7233"),
		TemporalWebURL:      os.Getenv("TEMPORAL_WEB_URL"),
		DashboardURL:        os.Getenv("DASHBOARD_URL"),
		WebhookPort:         getEnv("PORT", "8080"),
		GitHubWebhookSecret: getEnvOrFile("GITHUB_WEBHOOK_SECRET", "/etc/temporalci/github-webhook-secret"),
		GitHubToken:         getEnvOrFile("GITHUB_TOKEN", "/etc/temporalci/github-token"),
		LogBucket:           os.Getenv("LOG_BUCKET"),
		AWSRegion:           getEnv("AWS_REGION", "us-east-1"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvOrFile(envKey, filePath string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if data, err := os.ReadFile(filePath); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}
