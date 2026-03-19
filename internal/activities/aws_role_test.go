package activities

import "testing"

func TestAssumeRoleInput_Defaults(t *testing.T) {
	input := AssumeRoleInput{RoleARN: "arn:aws:iam::123456789012:role/test"}
	if input.Duration != 0 {
		t.Error("duration should default to 0 (set to 3600 in activity)")
	}
	if input.SessionName != "" {
		t.Error("session name should default to empty (set to 'temporalci' in activity)")
	}
}

func TestAssumeRoleInput_Chaining(t *testing.T) {
	input := AssumeRoleInput{
		RoleARN:            "arn:aws:iam::123456789012:role/upload",
		SourceAccessKey:    "AKIA...",
		SourceSecretKey:    "secret",
		SourceSessionToken: "token",
	}
	if input.SourceAccessKey == "" {
		t.Error("source credentials should be set for chaining")
	}
}
