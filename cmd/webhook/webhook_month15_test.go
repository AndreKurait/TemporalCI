package main

import (
	"strings"
	"testing"
)

// --- Month 15: Issue/PR lifecycle events, backport detection, badge endpoints ---

func TestParseIssuesEvent_Reopened(t *testing.T) {
	body := `{"action":"reopened","issue":{"number":55},"repository":{"full_name":"owner/repo"}}`
	inputs, err := parseEvent("issues", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(inputs))
	}
	if inputs[0].Parameters["ISSUE_ACTION"] != "reopened" {
		t.Errorf("action = %q", inputs[0].Parameters["ISSUE_ACTION"])
	}
	if inputs[0].Parameters["ISSUE_NUMBER"] != "55" {
		t.Errorf("number = %q", inputs[0].Parameters["ISSUE_NUMBER"])
	}
}

func TestParseIssuesEvent_Transferred(t *testing.T) {
	body := `{"action":"transferred","issue":{"number":77},"repository":{"full_name":"org/repo"}}`
	inputs, err := parseEvent("issues", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if inputs[0].Parameters["ISSUE_ACTION"] != "transferred" {
		t.Errorf("action = %q", inputs[0].Parameters["ISSUE_ACTION"])
	}
}

func TestParsePREvent_ClosedMerged_BackportBranch(t *testing.T) {
	body := `{"action":"closed","number":42,"pull_request":{"head":{"sha":"abc","ref":"backport/1.x-42"},"labels":[{"name":"backport-1.x"}],"merged":true},"repository":{"full_name":"owner/repo"}}`
	inputs, err := parseEvent("pull_request", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(inputs))
	}
	if inputs[0].Parameters["PR_ACTION"] != "closed" {
		t.Errorf("action = %q", inputs[0].Parameters["PR_ACTION"])
	}
	if inputs[0].Parameters["PR_MERGED"] != "true" {
		t.Errorf("merged = %q", inputs[0].Parameters["PR_MERGED"])
	}
	if inputs[0].Parameters["PR_HEAD_BRANCH"] != "backport/1.x-42" {
		t.Errorf("head branch = %q", inputs[0].Parameters["PR_HEAD_BRANCH"])
	}
	// Labels should be captured
	if len(inputs[0].Labels) != 1 || inputs[0].Labels[0] != "backport-1.x" {
		t.Errorf("labels = %v", inputs[0].Labels)
	}
}

func TestParsePREvent_Labeled(t *testing.T) {
	body := `{"action":"labeled","number":10,"pull_request":{"head":{"sha":"x","ref":"feat"},"labels":[{"name":"run-eks-tests"}],"merged":false},"repository":{"full_name":"o/r"}}`
	inputs, err := parseEvent("pull_request", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input for labeled, got %d", len(inputs))
	}
	if len(inputs[0].Labels) != 1 || inputs[0].Labels[0] != "run-eks-tests" {
		t.Errorf("labels = %v", inputs[0].Labels)
	}
}

func TestParseReleaseEvent_Published(t *testing.T) {
	body := `{"action":"published","release":{"tag_name":"v3.0","name":"Release 3.0","html_url":"https://github.com/o/r/releases/v3.0"},"repository":{"full_name":"owner/repo"}}`
	inputs, err := parseEvent("release", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if inputs[0].Event != "release" {
		t.Errorf("event = %q", inputs[0].Event)
	}
	if inputs[0].Parameters["TEMPORALCI_TAG"] != "v3.0" {
		t.Errorf("tag = %q", inputs[0].Parameters["TEMPORALCI_TAG"])
	}
	if inputs[0].Parameters["TEMPORALCI_RELEASE_NAME"] != "Release 3.0" {
		t.Errorf("name = %q", inputs[0].Parameters["TEMPORALCI_RELEASE_NAME"])
	}
}

func TestParsePushEvent_ChangedFiles(t *testing.T) {
	body := `{"ref":"refs/heads/main","after":"abc","repository":{"full_name":"o/r"},"commits":[{"added":["new.go"],"modified":["main.go"],"removed":["old.go"]},{"added":[],"modified":["main.go"],"removed":[]}]}`
	inputs, err := parseEvent("push", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	// Deduplicated: new.go, main.go, old.go
	if len(inputs[0].ChangedFiles) != 3 {
		t.Errorf("changed files = %v (expected 3 unique)", inputs[0].ChangedFiles)
	}
}

func TestBadgeSVG_ContainsRequiredElements(t *testing.T) {
	svg := badgeSVG("build", "passing", "#4c1")
	required := []string{"<svg", "build", "passing", "#4c1", "xmlns"}
	for _, r := range required {
		if !strings.Contains(svg, r) {
			t.Errorf("badge missing %q", r)
		}
	}
}

func TestBadgeSVG_DifferentLabelLengths(t *testing.T) {
	short := badgeSVG("ci", "ok", "#4c1")
	long := badgeSVG("continuous-integration", "passing-with-warnings", "#dfb317")
	if len(short) >= len(long) {
		t.Error("longer labels should produce longer SVG")
	}
}
