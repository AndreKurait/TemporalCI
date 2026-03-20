package workflows

import (
	"testing"
	"time"

	"github.com/AndreKurait/TemporalCI/internal/activities"
)

// --- Month 17: Release pipeline, credential chaining, analytics ---

func TestDepsOK(t *testing.T) {
	completed := map[string]bool{"build": true, "test": true}
	if !depsOK([]string{"build", "test"}, completed) {
		t.Error("all deps completed, should be OK")
	}
	if depsOK([]string{"build", "deploy"}, completed) {
		t.Error("deploy not completed, should fail")
	}
	if !depsOK(nil, completed) {
		t.Error("no deps should always be OK")
	}
	if !depsOK([]string{}, completed) {
		t.Error("empty deps should be OK")
	}
}

func TestResolveDynamicMatrix_ChainedOutputs(t *testing.T) {
	// Simulate credential chaining: step outputs AWS creds
	outputs := map[string]map[string]string{
		"assume-base": {
			"AWS_ACCESS_KEY_ID":     "AKIA...",
			"AWS_SECRET_ACCESS_KEY": "secret",
			"AWS_SESSION_TOKEN":     "token",
		},
	}
	// Verify outputs are accessible
	creds, ok := outputs["assume-base"]
	if !ok {
		t.Fatal("expected assume-base outputs")
	}
	if creds["AWS_ACCESS_KEY_ID"] != "AKIA..." {
		t.Errorf("access key = %q", creds["AWS_ACCESS_KEY_ID"])
	}
}

func TestBuildStepSecrets_WithAWSCreds(t *testing.T) {
	resolved := map[string]string{
		"DOCKER_PASSWORD":     "secret",
		"AWS_ACCESS_KEY_ID":   "AKIA...",
		"AWS_SECRET_ACCESS_KEY": "secret",
	}
	step := activities.StepConfig{
		Secrets: []string{"DOCKER_PASSWORD"},
	}
	got := buildStepSecrets(step, resolved)
	if got["DOCKER_PASSWORD"] != "secret" {
		t.Error("should include requested secret")
	}
	if _, ok := got["AWS_ACCESS_KEY_ID"]; ok {
		t.Error("should not include unrequested AWS creds")
	}
}

func TestParseTimeout_ReleasePipeline(t *testing.T) {
	// Release steps often have longer timeouts
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"30m", 30 * time.Minute},
		{"1h", time.Hour},
		{"2h30m", 2*time.Hour + 30*time.Minute},
		{"", 10 * time.Minute},
	}
	for _, tt := range tests {
		got := ParseTimeout(tt.input, 10*time.Minute)
		if got != tt.want {
			t.Errorf("ParseTimeout(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestMergeEnv_CredentialChaining(t *testing.T) {
	paramEnv := map[string]string{"event": "release", "TEMPORALCI_TAG": "v2.0"}
	awsCreds := map[string]string{
		"AWS_ACCESS_KEY_ID":     "AKIA...",
		"AWS_SECRET_ACCESS_KEY": "secret",
		"AWS_SESSION_TOKEN":     "token",
	}
	merged := mergeEnv(paramEnv, awsCreds)
	if merged["event"] != "release" {
		t.Error("should preserve param env")
	}
	if merged["AWS_ACCESS_KEY_ID"] != "AKIA..." {
		t.Error("should include AWS creds")
	}
	if merged["TEMPORALCI_TAG"] != "v2.0" {
		t.Error("should preserve tag")
	}
}

func TestFlattenOutputs_MultipleSteps(t *testing.T) {
	outputs := map[string]map[string]string{
		"build":       {"ARTIFACT_PATH": "/out/app.tar.gz"},
		"assume-role": {"AWS_ACCESS_KEY_ID": "AKIA..."},
		"sbom":        {"SBOM_PATH": "/out/sbom.json"},
	}
	flat := flattenOutputs(outputs)
	if flat["ARTIFACT_PATH"] != "/out/app.tar.gz" {
		t.Error("missing build output")
	}
	if flat["AWS_ACCESS_KEY_ID"] != "AKIA..." {
		t.Error("missing role output")
	}
	if flat["SBOM_PATH"] != "/out/sbom.json" {
		t.Error("missing sbom output")
	}
}

func TestCIPipelineInput_ReleaseFields(t *testing.T) {
	input := CIPipelineInput{
		Event:        "release",
		Repo:         "owner/repo",
		Ref:          "v2.0.0",
		PipelineName: "release",
		Parameters: map[string]string{
			"TEMPORALCI_TAG":          "v2.0.0",
			"TEMPORALCI_RELEASE_NAME": "Release 2.0.0",
		},
	}
	if input.Event != "release" {
		t.Errorf("event = %q", input.Event)
	}
	if input.Parameters["TEMPORALCI_TAG"] != "v2.0.0" {
		t.Errorf("tag = %q", input.Parameters["TEMPORALCI_TAG"])
	}
}

func TestCIPipelineResult_AggregatedSteps(t *testing.T) {
	result := CIPipelineResult{
		Status:       "passed",
		PipelineName: "release",
		Steps: []activities.StepResult{
			{Name: "approval", Status: "passed", Duration: 120},
			{Name: "build", Status: "passed", Duration: 180},
			{Name: "publish", Status: "passed", Duration: 60},
			{Name: "sbom", Status: "passed", Duration: 30},
			{Name: "release", Status: "passed", Duration: 15},
		},
	}
	if len(result.Steps) != 5 {
		t.Errorf("expected 5 steps, got %d", len(result.Steps))
	}
	var totalDuration float64
	for _, s := range result.Steps {
		totalDuration += s.Duration
	}
	if totalDuration != 405 {
		t.Errorf("total duration = %v, want 405", totalDuration)
	}
}
