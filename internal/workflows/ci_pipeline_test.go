package workflows

import (
	"testing"
	"time"
)

func TestParseTimeout(t *testing.T) {
	tests := []struct {
		input    string
		fallback time.Duration
		want     time.Duration
	}{
		{"5m", time.Minute, 5 * time.Minute},
		{"30s", time.Minute, 30 * time.Second},
		{"", time.Minute, time.Minute},
		{"invalid", 2 * time.Minute, 2 * time.Minute},
	}
	for _, tt := range tests {
		got := ParseTimeout(tt.input, tt.fallback)
		if got != tt.want {
			t.Errorf("ParseTimeout(%q, %v) = %v, want %v", tt.input, tt.fallback, got, tt.want)
		}
	}
}

func TestDepsOK(t *testing.T) {
	completed := map[string]bool{"build": true, "lint": true}

	if !depsOK([]string{"build"}, completed) {
		t.Error("depsOK should return true when dep is completed")
	}
	if !depsOK(nil, completed) {
		t.Error("depsOK should return true for no deps")
	}
	if depsOK([]string{"test"}, completed) {
		t.Error("depsOK should return false when dep is not completed")
	}
	if depsOK([]string{"build", "test"}, completed) {
		t.Error("depsOK should return false when any dep is not completed")
	}
}
