package main

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

// --- Helper functions for CI API logic (pure functions extracted for testability) ---

// buildStatusFromWorkflow maps Temporal workflow status + result to a CI build status string.
func buildStatusFromWorkflow(workflowStatus string, resultStatus string) string {
	switch workflowStatus {
	case "RUNNING":
		return "running"
	case "COMPLETED":
		if resultStatus == "passed" {
			return "passed"
		}
		return "failed"
	case "FAILED":
		return "failed"
	case "CANCELED":
		return "cancelled"
	case "TIMED_OUT":
		return "timed_out"
	default:
		return "unknown"
	}
}

// BuildFilters holds parsed query parameters for listing builds.
type BuildFilters struct {
	Repo   string
	Status string
	Limit  int
}

// parseBuildFilters extracts build list filters from URL query parameters.
func parseBuildFilters(params url.Values) BuildFilters {
	f := BuildFilters{
		Repo:   params.Get("repo"),
		Status: params.Get("status"),
		Limit:  50,
	}
	if v := params.Get("limit"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			f.Limit = n
		}
	}
	return f
}

// buildQueryString constructs a Temporal visibility query from filters.
func buildQueryString(f BuildFilters) string {
	clauses := []string{"WorkflowType='CIPipeline'"}
	if f.Repo != "" {
		clauses = append(clauses, fmt.Sprintf("Repo='%s'", f.Repo))
	}
	if f.Status != "" {
		clauses = append(clauses, fmt.Sprintf("Status='%s'", f.Status))
	}
	return strings.Join(clauses, " AND ")
}

// StepSummary holds counts for formatting a build summary.
type StepSummary struct {
	Total   int
	Passed  int
	Failed  int
	Running int
}

// formatBuildSummary produces a human-readable summary from step counts.
func formatBuildSummary(s StepSummary) string {
	if s.Running > 0 {
		return fmt.Sprintf("%d/%d passed, %d running", s.Passed, s.Total, s.Running)
	}
	if s.Failed > 0 {
		return fmt.Sprintf("%d/%d passed, %d failed", s.Passed, s.Total, s.Failed)
	}
	return fmt.Sprintf("%d/%d passed", s.Passed, s.Total)
}

// computeAnalytics calculates success rate and average duration from build data.
func computeAnalytics(statuses []string, durations []time.Duration) (successRate float64, avgDuration time.Duration) {
	if len(statuses) == 0 {
		return 0, 0
	}
	passed := 0
	for _, s := range statuses {
		if s == "passed" {
			passed++
		}
	}
	successRate = float64(passed) / float64(len(statuses)) * 100

	if len(durations) == 0 {
		return successRate, 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	avgDuration = total / time.Duration(len(durations))
	return
}

// --- Tests ---

func TestBuildStatusFromWorkflow(t *testing.T) {
	tests := []struct {
		name           string
		workflowStatus string
		resultStatus   string
		expected       string
	}{
		{"running", "RUNNING", "", "running"},
		{"completed_passed", "COMPLETED", "passed", "passed"},
		{"completed_failed", "COMPLETED", "failed", "failed"},
		{"failed", "FAILED", "", "failed"},
		{"canceled", "CANCELED", "", "cancelled"},
		{"timed_out", "TIMED_OUT", "", "timed_out"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildStatusFromWorkflow(tt.workflowStatus, tt.resultStatus)
			if got != tt.expected {
				t.Errorf("buildStatusFromWorkflow(%q, %q) = %q, want %q", tt.workflowStatus, tt.resultStatus, got, tt.expected)
			}
		})
	}
}

func TestParseBuildFilters(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		expectedRepo   string
		expectedStatus string
		expectedLimit  int
	}{
		{"empty", "", "", "", 50},
		{"repo", "repo=owner/repo", "owner/repo", "", 50},
		{"status", "status=failed", "", "failed", 50},
		{"limit_valid", "limit=10", "", "", 10},
		{"limit_invalid", "limit=invalid", "", "", 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, _ := url.ParseQuery(tt.query)
			got := parseBuildFilters(params)
			if got.Repo != tt.expectedRepo {
				t.Errorf("Repo = %q, want %q", got.Repo, tt.expectedRepo)
			}
			if got.Status != tt.expectedStatus {
				t.Errorf("Status = %q, want %q", got.Status, tt.expectedStatus)
			}
			if got.Limit != tt.expectedLimit {
				t.Errorf("Limit = %d, want %d", got.Limit, tt.expectedLimit)
			}
		})
	}
}

func TestBuildQueryString(t *testing.T) {
	tests := []struct {
		name     string
		filters  BuildFilters
		expected string
	}{
		{"no_filters", BuildFilters{Limit: 50}, "WorkflowType='CIPipeline'"},
		{"repo_filter", BuildFilters{Repo: "owner/repo"}, "WorkflowType='CIPipeline' AND Repo='owner/repo'"},
		{"status_filter", BuildFilters{Status: "failed"}, "WorkflowType='CIPipeline' AND Status='failed'"},
		{"multiple_filters", BuildFilters{Repo: "owner/repo", Status: "passed"}, "WorkflowType='CIPipeline' AND Repo='owner/repo' AND Status='passed'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildQueryString(tt.filters)
			if got != tt.expected {
				t.Errorf("buildQueryString() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFormatBuildSummary(t *testing.T) {
	tests := []struct {
		name     string
		summary  StepSummary
		expected string
	}{
		{"all_passed", StepSummary{Total: 5, Passed: 5}, "5/5 passed"},
		{"some_failed", StepSummary{Total: 5, Passed: 3, Failed: 2}, "3/5 passed, 2 failed"},
		{"running", StepSummary{Total: 5, Passed: 2, Running: 1}, "2/5 passed, 1 running"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBuildSummary(tt.summary)
			if got != tt.expected {
				t.Errorf("formatBuildSummary() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAnalyticsComputation(t *testing.T) {
	tests := []struct {
		name         string
		statuses     []string
		durations    []time.Duration
		expectedRate float64
		expectedAvg  time.Duration
	}{
		{"empty", nil, nil, 0, 0},
		{"all_passed", []string{"passed", "passed"}, []time.Duration{2 * time.Minute, 4 * time.Minute}, 100, 3 * time.Minute},
		{"mixed", []string{"passed", "failed", "passed"}, []time.Duration{1 * time.Minute, 2 * time.Minute, 3 * time.Minute}, 66.66666666666667, 2 * time.Minute},
		{"all_failed", []string{"failed", "failed"}, []time.Duration{5 * time.Minute, 5 * time.Minute}, 0, 5 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rate, avg := computeAnalytics(tt.statuses, tt.durations)
			if rate != tt.expectedRate {
				t.Errorf("successRate = %v, want %v", rate, tt.expectedRate)
			}
			if avg != tt.expectedAvg {
				t.Errorf("avgDuration = %v, want %v", avg, tt.expectedAvg)
			}
		})
	}
}
