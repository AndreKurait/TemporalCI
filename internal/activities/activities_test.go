package activities

import "testing"

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 100, "short"},
		{"", 10, ""},
		{"hello world", 5, "... (truncated)\nworld"},
		{"abcdefghij", 10, "abcdefghij"},
		{"abcdefghijk", 10, "... (truncated)\nbcdefghijk"},
	}
	for _, tt := range tests {
		got := TruncateOutput(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("TruncateOutput(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestWorkflowURL(t *testing.T) {
	tests := []struct {
		base, wfID, want string
	}{
		{
			"http://temporal.example.com",
			"ci-simple",
			"http://temporal.example.com/namespaces/default/workflows/ci-simple",
		},
		{
			"http://temporal.example.com",
			"ci-AndreKurait/TemporalCI-test-refs/heads/main-push",
			"http://temporal.example.com/namespaces/default/workflows/ci-AndreKurait%2FTemporalCI-test-refs%2Fheads%2Fmain-push",
		},
	}
	for _, tt := range tests {
		got := WorkflowURL(tt.base, tt.wfID)
		if got != tt.want {
			t.Errorf("WorkflowURL(%q, %q) = %q, want %q", tt.base, tt.wfID, got, tt.want)
		}
	}
}

func TestTrimRef(t *testing.T) {
	tests := []struct{ input, want string }{
		{"refs/heads/main", "main"},
		{"refs/tags/v1.0", "v1.0"},
		{"feature/test", "feature/test"},
	}
	for _, tt := range tests {
		got := trimRef(tt.input)
		if got != tt.want {
			t.Errorf("trimRef(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSplitRepo(t *testing.T) {
	owner, repo, err := splitRepo("AndreKurait/TemporalCI")
	if err != nil || owner != "AndreKurait" || repo != "TemporalCI" {
		t.Errorf("splitRepo failed: %s/%s err=%v", owner, repo, err)
	}

	_, _, err = splitRepo("invalid")
	if err == nil {
		t.Error("splitRepo should fail for invalid input")
	}
}
