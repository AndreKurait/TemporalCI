package activities

import (
	"testing"
)

func TestTruncateForCheckRun_Short(t *testing.T) {
	result := truncateForCheckRun("hello", 60000)
	if result != "```\nhello\n```" {
		t.Errorf("got %q", result)
	}
}

func TestTruncateForCheckRun_Long(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += "line " + string(rune('0'+i%10)) + "\n"
	}
	result := truncateForCheckRun(long, 200)
	if len(result) > 250 { // some overhead for markers
		t.Errorf("result too long: %d", len(result))
	}
	if result[:3] != "```" {
		t.Error("should start with code fence")
	}
}
