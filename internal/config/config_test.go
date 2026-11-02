package config

import "testing"

func TestWebhookBindsLoopbackByDefault(t *testing.T) {
	t.Setenv("WEBHOOK_HOST", "")
	if got := LoadConfig().WebhookHost; got != "127.0.0.1" {
		t.Fatalf("default webhook host = %q", got)
	}
	t.Setenv("WEBHOOK_HOST", "0.0.0.0")
	if got := LoadConfig().WebhookHost; got != "0.0.0.0" {
		t.Fatalf("explicit container bind = %q", got)
	}
}
