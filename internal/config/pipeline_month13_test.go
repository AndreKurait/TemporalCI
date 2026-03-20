package config

import (
	"os"
	"path/filepath"
	"testing"
)

// --- Month 13: Scheduled triggers, tag triggers, manual trigger parameters ---

func TestShouldRun_CronSchedule(t *testing.T) {
	p := &Pipeline{
		On: &TriggerConfig{
			Schedule: []ScheduleEntry{
				{Cron: "25 22 * * 0"},
			},
		},
	}
	if !p.ShouldRun("schedule", "") {
		t.Error("should run for schedule event")
	}
	if p.ShouldRun("push", "main") {
		t.Error("should not run for push when only schedule configured")
	}
}

func TestShouldRun_TagPush_Wildcard(t *testing.T) {
	p := &Pipeline{
		On: &TriggerConfig{
			Push: &PushFilter{Tags: []string{"*"}},
		},
	}
	if !p.ShouldRun("tag", "v1.0.0") {
		t.Error("should run for tag event with wildcard")
	}
	if !p.ShouldRun("push", "v2.0.0") {
		t.Error("should run for push with tag wildcard")
	}
}

func TestShouldRun_TagPush_Specific(t *testing.T) {
	p := &Pipeline{
		On: &TriggerConfig{
			Push: &PushFilter{Tags: []string{"v1.0.0"}},
		},
	}
	if !p.ShouldRun("tag", "v1.0.0") {
		t.Error("should run for matching tag")
	}
	if p.ShouldRun("tag", "v2.0.0") {
		t.Error("should not run for non-matching tag")
	}
}

func TestShouldRun_IssuesEvent(t *testing.T) {
	p := &Pipeline{
		On: &TriggerConfig{
			Issues: &EventFilter{Types: []string{"opened", "reopened"}},
		},
	}
	if !p.ShouldRun("issues", "") {
		t.Error("should run for issues event")
	}
	if p.ShouldRun("push", "main") {
		t.Error("should not run for push when only issues configured")
	}
}

func TestShouldRun_MultipleTriggers(t *testing.T) {
	p := &Pipeline{
		On: &TriggerConfig{
			Push:        &PushFilter{Branches: []string{"main"}},
			PullRequest: &BranchFilter{Branches: []string{"main"}},
			Schedule:    []ScheduleEntry{{Cron: "0 0 * * *"}},
		},
	}
	if !p.ShouldRun("push", "main") {
		t.Error("should run for push to main")
	}
	if !p.ShouldRun("pull_request", "main") {
		t.Error("should run for PR to main")
	}
	if !p.ShouldRun("schedule", "") {
		t.Error("should run for schedule")
	}
	if p.ShouldRun("push", "develop") {
		t.Error("should not run for push to develop")
	}
}

func TestResolveParameters_ManualTrigger(t *testing.T) {
	params := []ParameterConfig{
		{Name: "ENVIRONMENT", Type: "choice", Options: []string{"staging", "production"}, Default: "staging"},
		{Name: "DRY_RUN", Type: "boolean", Default: "true"},
		{Name: "VERSION", Type: "string"},
	}

	// Simulate manual trigger with overrides
	env, err := ResolveParameters(params, map[string]string{
		"ENVIRONMENT": "production",
		"DRY_RUN":     "false",
		"VERSION":     "v2.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if env["ENVIRONMENT"] != "production" {
		t.Errorf("ENVIRONMENT = %q", env["ENVIRONMENT"])
	}
	if env["DRY_RUN"] != "false" {
		t.Errorf("DRY_RUN = %q", env["DRY_RUN"])
	}
	if env["VERSION"] != "v2.0" {
		t.Errorf("VERSION = %q", env["VERSION"])
	}
}

func TestResolveParameters_DefaultsOnly(t *testing.T) {
	params := []ParameterConfig{
		{Name: "X", Type: "string", Default: "hello"},
	}
	env, err := ResolveParameters(params, nil)
	if err != nil {
		t.Fatal(err)
	}
	if env["X"] != "hello" {
		t.Errorf("X = %q, want hello", env["X"])
	}
}

func TestLoadPipelineConfig_WithSchedule(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "codeql", `on:
  schedule:
    - cron: "25 22 * * 0"
steps:
  - name: analyze
    image: temporalci/ci-codeql
    command: codeql database create --language=java
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Pipelines["codeql"]
	if p == nil || p.On == nil || len(p.On.Schedule) != 1 {
		t.Fatal("expected schedule config")
	}
	if p.On.Schedule[0].Cron != "25 22 * * 0" {
		t.Errorf("cron = %q", p.On.Schedule[0].Cron)
	}
}

func TestLoadPipelineConfig_WithParameters(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "deploy", `parameters:
  - name: ENV
    type: choice
    options: [staging, production]
    default: staging
  - name: SKIP_TESTS
    type: boolean
    default: "false"
steps:
  - name: deploy
    command: ./deploy.sh
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Pipelines["deploy"]
	if len(p.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(p.Parameters))
	}
	if p.Parameters[0].Type != "choice" || len(p.Parameters[0].Options) != 2 {
		t.Errorf("param 0: %+v", p.Parameters[0])
	}
}

func TestShouldRunWithPaths_PushPathFilter(t *testing.T) {
	p := &Pipeline{
		On: &TriggerConfig{
			Push: &PushFilter{
				Branches: []string{"main"},
				Paths:    []string{"src/**", "*.go"},
			},
		},
	}
	if !p.ShouldRunWithPaths("push", "main", []string{"src/main.go"}) {
		t.Error("should run for changed file matching src/**")
	}
	if p.ShouldRunWithPaths("push", "main", []string{"docs/readme.md"}) {
		t.Error("should not run for docs-only change")
	}
	if !p.ShouldRunWithPaths("push", "main", []string{"handler.go"}) {
		t.Error("should run for *.go match")
	}
}

func TestShouldRunWithPaths_NoPaths(t *testing.T) {
	p := &Pipeline{
		On: &TriggerConfig{
			Push: &PushFilter{Branches: []string{"main"}},
		},
	}
	// No path filter = always match
	if !p.ShouldRunWithPaths("push", "main", []string{"anything.txt"}) {
		t.Error("should run when no path filter")
	}
}

func TestMatchesChangedPaths(t *testing.T) {
	tests := []struct {
		patterns []string
		files    []string
		want     bool
	}{
		{[]string{"src/**"}, []string{"src/main.go"}, true},
		{[]string{"src/**"}, []string{"docs/readme.md"}, false},
		{[]string{"*.go"}, []string{"main.go"}, true},
		{[]string{"docs/*"}, []string{"docs/readme.md"}, true},
		{[]string{"docs/*"}, []string{"docs/sub/file.md"}, false},
		{nil, []string{"anything"}, true},
	}
	for _, tt := range tests {
		got := MatchesChangedPaths(tt.patterns, tt.files)
		if got != tt.want {
			t.Errorf("MatchesChangedPaths(%v, %v) = %v, want %v", tt.patterns, tt.files, got, tt.want)
		}
	}
}

func TestLoadPipelineConfig_PRWithLabels(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "eks-tests", `on:
  pull_request:
    labels: [run-eks-tests]
steps:
  - name: eks-test
    command: ./run-eks.sh
    if: "labels contains 'run-eks-tests'"
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Pipelines["eks-tests"]
	if p.On.PullRequest == nil || len(p.On.PullRequest.Labels) != 1 {
		t.Error("expected PR label filter")
	}
	if p.Steps[0].GetCondition() != "labels contains 'run-eks-tests'" {
		t.Errorf("condition = %q", p.Steps[0].GetCondition())
	}
}

func TestGetCommand_Commands(t *testing.T) {
	s := StepConfig{Commands: []string{"echo a", "echo b", "echo c"}}
	got := s.GetCommand()
	if got != "echo a && echo b && echo c" {
		t.Errorf("got %q", got)
	}
}

func TestGetCondition_IfAlias(t *testing.T) {
	s := StepConfig{If: "event == 'push'"}
	if s.GetCondition() != "event == 'push'" {
		t.Errorf("got %q", s.GetCondition())
	}
	s2 := StepConfig{When: "branch == 'main'"}
	if s2.GetCondition() != "branch == 'main'" {
		t.Errorf("got %q", s2.GetCondition())
	}
}

func TestValidate_EmptyMatrixAxis(t *testing.T) {
	dir := t.TempDir()
	pDir := filepath.Join(dir, ".temporalci")
	os.MkdirAll(pDir, 0755)
	os.WriteFile(filepath.Join(pDir, "ci.yaml"), []byte(`steps:
  - name: test
    matrix:
      index: []
`), 0644)
	cfg, _ := LoadPipelineConfig(dir)
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if contains(e, "empty") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected empty matrix axis error, got %v", errs)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
