package activities

import "testing"

// --- Month 17: AWS credential chaining, release artifacts ---

func TestAssumeRoleInput_ChainedCredentials(t *testing.T) {
	// First hop: Pod Identity → base role
	base := AssumeRoleInput{
		RoleARN:     "arn:aws:iam::123:role/base",
		SessionName: "temporalci-release",
		Duration:    3600,
	}
	if base.SourceAccessKey != "" {
		t.Error("first hop should not have source credentials")
	}

	// Second hop: base role → upload role
	upload := AssumeRoleInput{
		RoleARN:            "arn:aws:iam::123:role/upload",
		SessionName:        "temporalci-upload",
		Duration:           3600,
		SourceAccessKey:    "AKIA...",
		SourceSecretKey:    "secret",
		SourceSessionToken: "token",
	}
	if upload.SourceAccessKey == "" {
		t.Error("second hop should have source credentials")
	}
}

func TestAssumeRoleResult_Fields(t *testing.T) {
	result := AssumeRoleResult{
		AccessKeyID:     "ASIA...",
		SecretAccessKey: "newsecret",
		SessionToken:    "newtoken",
		Expiration:      "2026-01-01T00:00:00Z",
	}
	if result.AccessKeyID == "" || result.SecretAccessKey == "" || result.SessionToken == "" {
		t.Error("all credential fields should be set")
	}
}

func TestArtifactUploadInput_Fields(t *testing.T) {
	input := UploadArtifactInput{
		WorkflowID: "ci-release-v2",
		StepName:   "build",
		Repo:       "owner/repo",
		Paths: []ArtifactUpload{
			{Path: "/out/app.tar.gz"},
			{Path: "/out/sbom.json"},
		},
	}
	if len(input.Paths) != 2 {
		t.Errorf("paths = %d", len(input.Paths))
	}
}

func TestListArtifactsInput_Fields(t *testing.T) {
	input := ListArtifactsInput{
		Repo:       "owner/repo",
		WorkflowID: "ci-release-v2",
	}
	if input.Repo != "owner/repo" {
		t.Errorf("repo = %q", input.Repo)
	}
}

func TestNotifySlackInput_ReleaseFormat(t *testing.T) {
	input := NotifySlackInput{
		WebhookURL: "https://hooks.slack.com/services/xxx",
		Repo:       "owner/repo",
		Ref:        "refs/tags/v2.0.0",
		Status:     "passed",
		StepCount:  8,
		Duration:   405.0,
		WorkflowID: "ci-release-v2",
	}
	if input.StepCount != 8 {
		t.Errorf("step count = %d", input.StepCount)
	}
}
