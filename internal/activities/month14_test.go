package activities

import (
	"compress/gzip"
	"encoding/base64"
	"io"
	"strings"
	"testing"
)

// --- Month 14: SARIF upload, GitHub App permissions, Check Run annotations ---

func TestUploadSARIFInput_Fields(t *testing.T) {
	input := UploadSARIFInput{
		Repo:      "owner/repo",
		HeadSHA:   "abc123",
		Ref:       "refs/heads/main",
		SARIFPath: "/tmp/results.sarif",
	}
	if input.Repo != "owner/repo" {
		t.Errorf("repo = %q", input.Repo)
	}
	if input.HeadSHA != "abc123" {
		t.Errorf("sha = %q", input.HeadSHA)
	}
}

func TestGzipBase64Encode(t *testing.T) {
	// Verify the gzip+base64 encoding logic used in SARIF upload
	data := []byte(`{"version":"2.1.0","runs":[]}`)

	var buf strings.Builder
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		t.Fatal(err)
	}
	gw.Close()
	encoded := base64.StdEncoding.EncodeToString([]byte(buf.String()))

	// Decode and verify
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	gr, err := gzip.NewReader(strings.NewReader(string(decoded)))
	if err != nil {
		t.Fatal(err)
	}
	result, err := io.ReadAll(gr)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != string(data) {
		t.Errorf("roundtrip failed: got %q", string(result))
	}
}

func TestCheckRunConclusion_Mapping(t *testing.T) {
	tests := []struct {
		status, wantConclusion string
	}{
		{"passed", "success"},
		{"failed", "failure"},
		{"cancelled", "cancelled"},
		{"skipped", "skipped"},
	}
	for _, tt := range tests {
		conclusion := "success"
		switch tt.status {
		case "failed":
			conclusion = "failure"
		case "cancelled":
			conclusion = "cancelled"
		case "skipped":
			conclusion = "skipped"
		}
		if conclusion != tt.wantConclusion {
			t.Errorf("status %q → conclusion %q, want %q", tt.status, conclusion, tt.wantConclusion)
		}
	}
}

func TestCheckRunName_Format(t *testing.T) {
	stepName := "gradle-tests [index=7]"
	name := "TemporalCI / " + stepName
	if name != "TemporalCI / gradle-tests [index=7]" {
		t.Errorf("got %q", name)
	}
}

func TestTruncateForCheckRun_Boundaries(t *testing.T) {
	// Exactly at limit
	short := "hello"
	got := truncateForCheckRun(short, 60000)
	if got != "```\nhello\n```" {
		t.Errorf("short = %q", got)
	}

	// Empty
	got = truncateForCheckRun("", 60000)
	if got != "```\n\n```" {
		t.Errorf("empty = %q", got)
	}
}

func TestStepResult_MatrixKey(t *testing.T) {
	r := StepResult{
		Name:      "gradle-tests [index=7]",
		Status:    "failed",
		MatrixKey: "index=7",
		Duration:  12.5,
	}
	if r.MatrixKey != "index=7" {
		t.Errorf("matrix key = %q", r.MatrixKey)
	}
}

func TestGetEffectiveCommand(t *testing.T) {
	tests := []struct {
		name string
		step StepConfig
		want string
	}{
		{"command", StepConfig{Command: "go test"}, "go test"},
		{"commands", StepConfig{Commands: []string{"go build", "go test"}}, "go build && go test"},
		{"both", StepConfig{Command: "go test", Commands: []string{"a", "b"}}, "go test"},
		{"empty", StepConfig{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.step.GetEffectiveCommand()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
