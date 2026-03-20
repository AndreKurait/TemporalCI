package activities

import "testing"

// --- Month 12: Dashboard URL generation and Check Run deep linking ---

func TestDashboardBuildURL(t *testing.T) {
	tests := []struct {
		base, wfID, want string
	}{
		{
			"https://ci.example.com",
			"ci-owner/repo-main-push",
			"https://ci.example.com/ci/builds/ci-owner%2Frepo-main-push",
		},
		{
			"https://ci.example.com",
			"simple-id",
			"https://ci.example.com/ci/builds/simple-id",
		},
	}
	for _, tt := range tests {
		got := DashboardBuildURL(tt.base, tt.wfID)
		if got != tt.want {
			t.Errorf("DashboardBuildURL(%q, %q) = %q, want %q", tt.base, tt.wfID, got, tt.want)
		}
	}
}

func TestCheckRunDetailsURL_FailedStep(t *testing.T) {
	base := "https://ci.example.com"
	wfID := "ci-owner/repo-main-push"
	step := StepResult{Name: "gradle-tests", Status: "failed"}

	url := DashboardBuildURL(base, wfID)
	if step.Status == "failed" {
		url += "#step-" + step.Name
	}
	want := "https://ci.example.com/ci/builds/ci-owner%2Frepo-main-push#step-gradle-tests"
	if url != want {
		t.Errorf("got %q, want %q", url, want)
	}
}

func TestCheckRunDetailsURL_PassedStep(t *testing.T) {
	base := "https://ci.example.com"
	wfID := "ci-simple"
	step := StepResult{Name: "test", Status: "passed"}

	url := DashboardBuildURL(base, wfID)
	if step.Status == "failed" {
		url += "#step-" + step.Name
	}
	// Passed steps don't get fragment
	want := "https://ci.example.com/ci/builds/ci-simple"
	if url != want {
		t.Errorf("got %q, want %q", url, want)
	}
}
