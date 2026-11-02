package config

import "testing"

func TestEventMetadataFilters(t *testing.T) {
	for _, tc := range []struct {
		name, event, ref, payload string
		on                        *TriggerConfig
		files, labels             []string
		want                      bool
	}{
		{name: "PR base branch and labels", event: "pull_request", ref: "feature",
			payload: `{"pull_request":{"base":{"ref":"main"}}}`,
			on:      &TriggerConfig{PullRequest: &BranchFilter{Branches: []string{"main"}, Labels: []string{"ci-run"}}},
			labels:  []string{"ci-run"}, want: true},
		{name: "PR missing required label", event: "pull_request", ref: "main",
			on: &TriggerConfig{PullRequest: &BranchFilter{Labels: []string{"ci-run"}}}},
		{name: "PR missing base cannot match source branch", event: "pull_request", ref: "main",
			on: &TriggerConfig{PullRequest: &BranchFilter{Branches: []string{"main"}}}},
		{name: "PR empty base cannot match source branch", event: "pull_request", ref: "main",
			payload: `{"pull_request":{"base":{"ref":""}}}`,
			on:      &TriggerConfig{PullRequest: &BranchFilter{Branches: []string{"main"}}}},
		{name: "PR malformed payload cannot match source branch", event: "pull_request", ref: "main",
			payload: `{"pull_request":`,
			on:      &TriggerConfig{PullRequest: &BranchFilter{Branches: []string{"main"}}}},
		{name: "PR unavailable changed paths", event: "pull_request", ref: "main",
			on: &TriggerConfig{PullRequest: &BranchFilter{Paths: []string{"src/**"}}}},
		{name: "release action mismatch", event: "release", payload: `{"action":"deleted"}`,
			on: &TriggerConfig{Release: &ReleaseFilter{Types: []string{"published"}}}},
		{name: "release action matches", event: "release", payload: `{"action":"published"}`,
			on: &TriggerConfig{Release: &ReleaseFilter{Types: []string{"published"}}}, want: true},
		{name: "custom payload mismatch", event: "webhook", payload: `{"build":{"status":"failed"}}`,
			on: &TriggerConfig{Webhook: &WebhookMatch{Match: map[string]string{"build.status": "passed"}}}},
		{name: "custom payload matches", event: "webhook", payload: `{"build":{"status":"passed"}}`,
			on: &TriggerConfig{Webhook: &WebhookMatch{Match: map[string]string{"build.status": "passed"}}}, want: true},
		{name: "tag pattern", event: "tag", ref: "refs/tags/v1.2.3",
			on: &TriggerConfig{Push: &PushFilter{Tags: []string{"v*"}}}, want: true},
		{name: "branch cannot satisfy tag pattern", event: "push", ref: "refs/heads/v1",
			on: &TriggerConfig{Push: &PushFilter{Tags: []string{"v*"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Pipeline{On: tc.on}
			if got := p.ShouldRunForEvent(tc.event, tc.ref, tc.payload, tc.files, tc.labels); got != tc.want {
				t.Fatalf("ShouldRunForEvent = %v, want %v", got, tc.want)
			}
		})
	}
}
