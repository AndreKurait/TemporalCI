package main

import (
	"net/url"
	"testing"
	"time"
)

// --- Month 12: CI Dashboard v1 tests ---
// Tests for CI API layer, build list/detail, badge generation, analytics computation.

func TestCIStatusToTemporalStatus(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"running", "Running"},
		{"completed", "Completed"},
		{"passed", "Completed"},
		{"failed", "Completed"},
		{"cancelled", "Canceled"},
		{"timed_out", "TimedOut"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := ciStatusToTemporalStatus(tt.input)
		if got != tt.want {
			t.Errorf("ciStatusToTemporalStatus(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildFilters_LimitBounds(t *testing.T) {
	tests := []struct {
		query string
		want  int
	}{
		{"limit=0", 50},
		{"limit=-1", 50},
		{"limit=201", 50},
		{"limit=100", 100},
		{"limit=1", 1},
		{"limit=200", 200},
		{"", 50},
	}
	for _, tt := range tests {
		params, _ := url.ParseQuery(tt.query)
		got := parseBuildFilters(params)
		if got.Limit != tt.want {
			t.Errorf("query=%q: limit=%d, want %d", tt.query, got.Limit, tt.want)
		}
	}
}

func TestBuildQueryString_AllFilters(t *testing.T) {
	f := BuildFilters{Repo: "owner/repo", Status: "failed"}
	got := buildQueryString(f)
	if got != "WorkflowType='CIPipeline' AND Repo='owner/repo' AND Status='failed'" {
		t.Errorf("got %q", got)
	}
}

func TestBuildStatusFromWorkflow_AllStates(t *testing.T) {
	tests := []struct {
		wfStatus, resultStatus, want string
	}{
		{"RUNNING", "", "running"},
		{"COMPLETED", "passed", "passed"},
		{"COMPLETED", "failed", "failed"},
		{"COMPLETED", "", "failed"},
		{"FAILED", "", "failed"},
		{"CANCELED", "", "cancelled"},
		{"TIMED_OUT", "", "timed_out"},
		{"UNKNOWN", "", "unknown"},
	}
	for _, tt := range tests {
		got := buildStatusFromWorkflow(tt.wfStatus, tt.resultStatus)
		if got != tt.want {
			t.Errorf("buildStatusFromWorkflow(%q, %q) = %q, want %q", tt.wfStatus, tt.resultStatus, got, tt.want)
		}
	}
}

func TestFormatBuildSummary_AllCases(t *testing.T) {
	tests := []struct {
		summary  StepSummary
		expected string
	}{
		{StepSummary{Total: 8, Passed: 8}, "8/8 passed"},
		{StepSummary{Total: 8, Passed: 6, Failed: 2}, "6/8 passed, 2 failed"},
		{StepSummary{Total: 8, Passed: 5, Running: 3}, "5/8 passed, 3 running"},
		{StepSummary{Total: 0}, "0/0 passed"},
		{StepSummary{Total: 30, Passed: 29, Failed: 1}, "29/30 passed, 1 failed"},
	}
	for _, tt := range tests {
		got := formatBuildSummary(tt.summary)
		if got != tt.expected {
			t.Errorf("formatBuildSummary(%+v) = %q, want %q", tt.summary, got, tt.expected)
		}
	}
}

func TestAnalyticsComputation_EdgeCases(t *testing.T) {
	// Single build
	rate, avg := computeAnalytics([]string{"passed"}, []time.Duration{3 * time.Minute})
	if rate != 100 {
		t.Errorf("single passed: rate=%v, want 100", rate)
	}
	if avg != 3*time.Minute {
		t.Errorf("single passed: avg=%v, want 3m", avg)
	}

	// No durations
	rate, avg = computeAnalytics([]string{"passed", "failed"}, nil)
	if rate != 50 {
		t.Errorf("no durations: rate=%v, want 50", rate)
	}
	if avg != 0 {
		t.Errorf("no durations: avg=%v, want 0", avg)
	}
}

func TestTopNStepCounts(t *testing.T) {
	counts := map[string]int{"a": 5, "b": 3, "c": 10, "d": 1}
	got := topNStepCounts(counts, 2)
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}
	if got[0].Name != "c" || got[0].Count != 10 {
		t.Errorf("first=%+v, want c:10", got[0])
	}
	if got[1].Name != "a" || got[1].Count != 5 {
		t.Errorf("second=%+v, want a:5", got[1])
	}
}

func TestTopNStepDurations(t *testing.T) {
	durations := map[string][]float64{
		"fast": {1.0, 2.0},
		"slow": {10.0, 20.0},
	}
	got := topNStepDurations(durations, 1)
	if len(got) != 1 {
		t.Fatalf("len=%d, want 1", len(got))
	}
	if got[0].Name != "slow" {
		t.Errorf("got %q, want slow", got[0].Name)
	}
	if got[0].AvgDuration != 15.0 {
		t.Errorf("avg=%v, want 15", got[0].AvgDuration)
	}
}

func TestTopNStepCounts_LessThanN(t *testing.T) {
	counts := map[string]int{"a": 1}
	got := topNStepCounts(counts, 5)
	if len(got) != 1 {
		t.Errorf("len=%d, want 1", len(got))
	}
}

func TestBadgeSVG_AllStatuses(t *testing.T) {
	statuses := []struct {
		label, status, color string
	}{
		{"build", "passing", "#4c1"},
		{"build", "failing", "#e05d44"},
		{"build", "running", "#dfb317"},
		{"build", "unknown", "#9f9f9f"},
	}
	for _, tt := range statuses {
		svg := badgeSVG(tt.label, tt.status, tt.color)
		if len(svg) == 0 {
			t.Errorf("empty SVG for %s/%s", tt.label, tt.status)
		}
	}
}
