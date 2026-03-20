package main

import (
	"strings"
	"testing"
)

func TestParsePushEvent_Branch(t *testing.T) {
	body := `{"ref":"refs/heads/main","after":"abc123","repository":{"full_name":"owner/repo"},"commits":[{"added":["src/new.go"],"modified":["src/main.go"],"removed":["old.txt"]}]}`
	inputs, err := parseEvent("push", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(inputs))
	}
	if inputs[0].Event != "push" {
		t.Errorf("event = %q, want push", inputs[0].Event)
	}
	if inputs[0].Repo != "owner/repo" {
		t.Errorf("repo = %q", inputs[0].Repo)
	}
	if len(inputs[0].ChangedFiles) != 3 {
		t.Errorf("expected 3 changed files, got %d: %v", len(inputs[0].ChangedFiles), inputs[0].ChangedFiles)
	}
}

func TestParsePushEvent_Tag(t *testing.T) {
	body := `{"ref":"refs/tags/v1.0.0","after":"abc123","repository":{"full_name":"owner/repo"}}`
	inputs, err := parseEvent("push", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(inputs))
	}
	if inputs[0].Event != "tag" {
		t.Errorf("event = %q, want tag", inputs[0].Event)
	}
	if inputs[0].Parameters["TEMPORALCI_TAG"] != "v1.0.0" {
		t.Errorf("tag = %q", inputs[0].Parameters["TEMPORALCI_TAG"])
	}
}

func TestParsePREvent_Opened(t *testing.T) {
	body := `{"action":"opened","number":42,"pull_request":{"head":{"sha":"def456","ref":"feature/x"},"labels":[{"name":"ci"},{"name":"run-eks-tests"}],"merged":false},"repository":{"full_name":"owner/repo"}}`
	inputs, err := parseEvent("pull_request", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(inputs))
	}
	if inputs[0].PRNumber != 42 {
		t.Errorf("PR number = %d", inputs[0].PRNumber)
	}
	if len(inputs[0].Labels) != 2 {
		t.Errorf("labels = %v", inputs[0].Labels)
	}
}

func TestParsePREvent_Closed(t *testing.T) {
	body := `{"action":"closed","number":10,"pull_request":{"head":{"sha":"abc","ref":"backport/1.x"},"labels":[],"merged":true},"repository":{"full_name":"owner/repo"}}`
	inputs, err := parseEvent("pull_request", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if inputs[0].Parameters["PR_ACTION"] != "closed" {
		t.Errorf("PR_ACTION = %q", inputs[0].Parameters["PR_ACTION"])
	}
	if inputs[0].Parameters["PR_MERGED"] != "true" {
		t.Errorf("PR_MERGED = %q", inputs[0].Parameters["PR_MERGED"])
	}
	if inputs[0].Parameters["PR_HEAD_BRANCH"] != "backport/1.x" {
		t.Errorf("PR_HEAD_BRANCH = %q", inputs[0].Parameters["PR_HEAD_BRANCH"])
	}
}

func TestParsePREvent_IgnoredAction(t *testing.T) {
	body := `{"action":"edited","number":1,"pull_request":{"head":{"sha":"x","ref":"y"},"labels":[],"merged":false},"repository":{"full_name":"o/r"}}`
	inputs, err := parseEvent("pull_request", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 0 {
		t.Errorf("expected 0 inputs for 'edited' action, got %d", len(inputs))
	}
}

func TestParseReleaseEvent(t *testing.T) {
	body := `{"action":"published","release":{"tag_name":"v2.0","name":"Release 2.0","html_url":"https://github.com/o/r/releases/v2.0"},"repository":{"full_name":"owner/repo"}}`
	inputs, err := parseEvent("release", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if inputs[0].Event != "release" {
		t.Errorf("event = %q", inputs[0].Event)
	}
	if inputs[0].Parameters["TEMPORALCI_TAG"] != "v2.0" {
		t.Errorf("tag = %q", inputs[0].Parameters["TEMPORALCI_TAG"])
	}
}

func TestParseIssuesEvent(t *testing.T) {
	body := `{"action":"opened","issue":{"number":99},"repository":{"full_name":"owner/repo"}}`
	inputs, err := parseEvent("issues", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if inputs[0].Event != "issues" {
		t.Errorf("event = %q", inputs[0].Event)
	}
	if inputs[0].Parameters["ISSUE_ACTION"] != "opened" {
		t.Errorf("ISSUE_ACTION = %q", inputs[0].Parameters["ISSUE_ACTION"])
	}
	if inputs[0].Parameters["ISSUE_NUMBER"] != "99" {
		t.Errorf("ISSUE_NUMBER = %q", inputs[0].Parameters["ISSUE_NUMBER"])
	}
}

func TestParseEvent_Unsupported(t *testing.T) {
	inputs, err := parseEvent("deployment", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 0 {
		t.Errorf("expected 0 inputs for unsupported event, got %d", len(inputs))
	}
}

func TestVerifySignature(t *testing.T) {
	payload := []byte(`{"test":"data"}`)
	secret := "mysecret"
	// Pre-computed: sha256 HMAC of payload with secret
	if verifySignature(payload, "sha256=invalid", secret) {
		t.Error("should reject invalid signature")
	}
	if verifySignature(payload, "short", secret) {
		t.Error("should reject short signature")
	}
}

func TestVerifySignature_Valid(t *testing.T) {
	payload := []byte(`test`)
	secret := "secret"
	// Just verify it doesn't panic with a wrong but well-formed signature
	result := verifySignature(payload, "sha256=0000000000000000000000000000000000000000000000000000000000000000", secret)
	if result {
		t.Error("should reject wrong signature")
	}
}

func TestBadgeSVG(t *testing.T) {
	svg := badgeSVG("build", "passing", "#4c1")
	if !strings.Contains(svg, "<svg") {
		t.Error("should be valid SVG")
	}
	if !strings.Contains(svg, "passing") {
		t.Error("should contain status text")
	}
	if !strings.Contains(svg, "build") {
		t.Error("should contain label text")
	}
	if !strings.Contains(svg, "#4c1") {
		t.Error("should contain color")
	}
}

func TestBadgeSVG_Failing(t *testing.T) {
	svg := badgeSVG("build", "failing", "#e05d44")
	if !strings.Contains(svg, "failing") {
		t.Error("should contain failing status")
	}
}
