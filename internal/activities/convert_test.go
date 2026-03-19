package activities

import (
	"testing"

	"github.com/AndreKurait/TemporalCI/internal/config"
)

func TestConvertStepConfig_Basic(t *testing.T) {
	src := config.StepConfig{
		Name:    "build",
		Image:   "golang:1.24",
		Command: "go build ./...",
		Timeout: "5m",
		Secrets: []string{"TOKEN"},
	}
	got := convertStepConfig(src)
	if got.Name != "build" || got.Image != "golang:1.24" || got.Command != "go build ./..." {
		t.Errorf("basic fields: %+v", got)
	}
	if got.Timeout != "5m" {
		t.Errorf("timeout = %q", got.Timeout)
	}
	if len(got.Secrets) != 1 || got.Secrets[0] != "TOKEN" {
		t.Errorf("secrets = %v", got.Secrets)
	}
}

func TestConvertStepConfig_Matrix(t *testing.T) {
	ff := false
	src := config.StepConfig{
		Name: "test",
		Matrix: &config.MatrixConfig{
			Dimensions:  map[string][]string{"index": {"0", "1", "2"}},
			MaxParallel: 5,
			FailFast:    &ff,
		},
	}
	got := convertStepConfig(src)
	if got.Matrix == nil {
		t.Fatal("matrix should not be nil")
	}
	if len(got.Matrix.Dimensions["index"]) != 3 {
		t.Errorf("dimensions = %v", got.Matrix.Dimensions)
	}
	if got.Matrix.MaxParallel != 5 {
		t.Errorf("max_parallel = %d", got.Matrix.MaxParallel)
	}
	if got.Matrix.FailFast != false {
		t.Error("fail_fast should be false")
	}
}

func TestConvertStepConfig_Services(t *testing.T) {
	src := config.StepConfig{
		Name:   "e2e",
		Docker: true,
		Services: []config.ServiceConfig{
			{
				Name:  "postgres",
				Image: "postgres:16",
				Ports: []int{5432},
				Health: &config.HealthCheck{
					Cmd:     "pg_isready",
					Retries: 30,
				},
			},
		},
	}
	got := convertStepConfig(src)
	if !got.Docker {
		t.Error("docker should be true")
	}
	if len(got.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(got.Services))
	}
	if got.Services[0].Health == nil || got.Services[0].Health.Retries != 30 {
		t.Error("health check not converted")
	}
}

func TestConvertStepConfig_AWSRole(t *testing.T) {
	src := config.StepConfig{
		Name: "deploy",
		AWSRole: &config.AWSRoleConfig{
			ARN:               "arn:aws:iam::123:role/deploy",
			Duration:          3600,
			SourceCredentials: "assume-base",
		},
	}
	got := convertStepConfig(src)
	if got.AWSRole == nil {
		t.Fatal("aws_role should not be nil")
	}
	if got.AWSRole.SourceCredentials != "assume-base" {
		t.Errorf("source_credentials = %q", got.AWSRole.SourceCredentials)
	}
}

func TestConvertStepConfig_Trigger(t *testing.T) {
	wait := true
	propagate := false
	src := config.StepConfig{
		Name: "child",
		Trigger: &config.TriggerStep{
			Pipeline:         "deploy",
			Parameters:       map[string]string{"ENV": "prod"},
			Wait:             &wait,
			PropagateFailure: &propagate,
		},
	}
	got := convertStepConfig(src)
	if got.Trigger == nil {
		t.Fatal("trigger should not be nil")
	}
	if got.Trigger.Pipeline != "deploy" {
		t.Errorf("pipeline = %q", got.Trigger.Pipeline)
	}
	if !got.Trigger.Wait {
		t.Error("wait should be true")
	}
	if got.Trigger.PropagateFailure {
		t.Error("propagate_failure should be false")
	}
}

func TestConvertStepConfig_Artifacts(t *testing.T) {
	src := config.StepConfig{
		Name: "test",
		Artifacts: &config.ArtifactConfig{
			Upload:   []config.ArtifactUpload{{Path: "/out/report.xml"}},
			Download: []config.ArtifactDownload{{FromStep: "build", Path: "/in/"}},
		},
	}
	got := convertStepConfig(src)
	if got.Artifacts == nil {
		t.Fatal("artifacts should not be nil")
	}
	if len(got.Artifacts.Upload) != 1 || got.Artifacts.Upload[0].Path != "/out/report.xml" {
		t.Errorf("upload = %v", got.Artifacts.Upload)
	}
	if len(got.Artifacts.Download) != 1 || got.Artifacts.Download[0].FromStep != "build" {
		t.Errorf("download = %v", got.Artifacts.Download)
	}
}

func TestConvertStepConfig_PostCommands(t *testing.T) {
	src := config.StepConfig{
		Name: "test",
		Post: []string{"codecov --flag gradle", "cleanup.sh"},
	}
	got := convertStepConfig(src)
	if len(got.Post) != 2 {
		t.Errorf("post = %v", got.Post)
	}
}
