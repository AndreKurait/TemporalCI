package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPipelineConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := `steps:
  - name: build
    image: golang:1.23
    command: go build ./...
    timeout: 5m
  - name: test
    image: golang:1.23
    command: go test -v ./...
    depends_on: [build]
    resources:
      cpu: "2"
      memory: 4Gi
`
	if err := os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(cfg.Steps))
	}
	if cfg.Steps[0].Timeout != "5m" {
		t.Errorf("step 0 timeout = %q, want 5m", cfg.Steps[0].Timeout)
	}
	if cfg.Steps[1].Resources == nil || cfg.Steps[1].Resources.CPU != "2" {
		t.Errorf("step 1 resources.cpu = %v, want 2", cfg.Steps[1].Resources)
	}
}

func TestLoadPipelineConfig_Missing(t *testing.T) {
	_, err := LoadPipelineConfig(t.TempDir())
	if err == nil {
		t.Error("expected error for missing config")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.Steps) != 2 {
		t.Fatalf("expected 2 default steps, got %d", len(cfg.Steps))
	}
}

func TestShouldRun_NoFilter(t *testing.T) {
	cfg := &PipelineConfig{}
	if !cfg.ShouldRun("push", "main") {
		t.Error("should run when no filter")
	}
}

func TestShouldRun_PushFilter(t *testing.T) {
	cfg := &PipelineConfig{
		On: &TriggerConfig{
			Push: &PushFilter{Branches: []string{"main", "develop"}},
		},
	}
	if !cfg.ShouldRun("push", "main") {
		t.Error("should run for push to main")
	}
	if cfg.ShouldRun("push", "feature/x") {
		t.Error("should not run for push to feature/x")
	}
	if cfg.ShouldRun("pull_request", "main") {
		t.Error("should not run for PR when only push configured")
	}
}

func TestShouldRun_PRFilter(t *testing.T) {
	cfg := &PipelineConfig{
		On: &TriggerConfig{
			PullRequest: &BranchFilter{Branches: []string{"main"}},
		},
	}
	if !cfg.ShouldRun("pull_request", "main") {
		t.Error("should run for PR to main")
	}
}

func TestShouldRun_TagFilter(t *testing.T) {
	cfg := &PipelineConfig{
		On: &TriggerConfig{
			Push: &PushFilter{Tags: []string{"*"}},
		},
	}
	if !cfg.ShouldRun("push", "v1.0.0") {
		t.Error("should run for tag push with wildcard")
	}
}

func TestShouldRun_Schedule(t *testing.T) {
	cfg := &PipelineConfig{
		On: &TriggerConfig{
			Schedule: []ScheduleEntry{{Cron: "0 22 * * *"}},
		},
	}
	if !cfg.ShouldRun("schedule", "") {
		t.Error("should run for schedule event")
	}
}

func TestShouldRun_Release(t *testing.T) {
	cfg := &PipelineConfig{
		On: &TriggerConfig{
			Release: &ReleaseFilter{Types: []string{"published"}},
		},
	}
	if !cfg.ShouldRun("release", "") {
		t.Error("should run for release event")
	}
}

func TestMatchingEnvironments(t *testing.T) {
	cfg := &PipelineConfig{
		Environments: map[string]*EnvConfig{
			"staging": {
				On:    &TriggerConfig{Push: &PushFilter{Branches: []string{"main"}}},
				Steps: []StepConfig{{Name: "deploy-staging", Command: "deploy"}},
			},
			"production": {
				On:       &TriggerConfig{Push: &PushFilter{Branches: []string{"release"}}},
				Approval: true,
				Steps:    []StepConfig{{Name: "deploy-prod", Command: "deploy"}},
			},
		},
	}

	envs := cfg.MatchingEnvironments("push", "main")
	if len(envs) != 1 {
		t.Fatalf("expected 1 matching env, got %d", len(envs))
	}
	if _, ok := envs["staging"]; !ok {
		t.Error("staging should match push to main")
	}

	envs = cfg.MatchingEnvironments("push", "release")
	if _, ok := envs["production"]; !ok {
		t.Error("production should match push to release")
	}
}

func TestPipelineConfig_Secrets(t *testing.T) {
	dir := t.TempDir()
	yaml := `steps:
  - name: deploy
    image: alpine
    command: deploy.sh
    secrets:
      - DOCKER_PASSWORD
      - NPM_TOKEN
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Steps[0].Secrets) != 2 {
		t.Errorf("expected 2 secrets, got %d", len(cfg.Steps[0].Secrets))
	}
	if cfg.Steps[0].Secrets[0] != "DOCKER_PASSWORD" {
		t.Errorf("secret[0] = %q, want DOCKER_PASSWORD", cfg.Steps[0].Secrets[0])
	}
}

func TestPipelineConfig_FullYAML(t *testing.T) {
	dir := t.TempDir()
	yaml := `on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

steps:
  - name: build
    image: golang:1.23
    command: go build ./...
  - name: test
    image: golang:1.23
    command: go test ./...
    depends_on: [build]

environments:
  staging:
    on:
      push:
        branches: [main]
    steps:
      - name: deploy-staging
        image: alpine
        command: helm upgrade staging
        secrets: [KUBE_TOKEN]
  production:
    on:
      push:
        branches: [release]
    approval: true
    steps:
      - name: deploy-prod
        image: alpine
        command: helm upgrade prod
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ShouldRun("push", "main") {
		t.Error("should run for push to main")
	}
	if len(cfg.Environments) != 2 {
		t.Errorf("expected 2 environments, got %d", len(cfg.Environments))
	}
	if !cfg.Environments["production"].Approval {
		t.Error("production should require approval")
	}
}

func TestMultiPipelineConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := `pipelines:
  eks-integ-test:
    on:
      push:
        branches: [main]
    steps:
      - name: test
        command: ./run-eks-test.sh
  nightly:
    on:
      schedule:
        - cron: "0 22 * * *"
    steps:
      - name: matrix
        command: ./run-matrix.sh
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	pipelines := cfg.GetPipelines()
	if len(pipelines) != 2 {
		t.Fatalf("expected 2 pipelines, got %d", len(pipelines))
	}
	if _, ok := pipelines["eks-integ-test"]; !ok {
		t.Error("missing eks-integ-test pipeline")
	}
	if _, ok := pipelines["nightly"]; !ok {
		t.Error("missing nightly pipeline")
	}
	if len(pipelines["nightly"].On.Schedule) != 1 {
		t.Error("nightly should have 1 schedule")
	}
}

func TestGetPipelines_FlatFormat(t *testing.T) {
	cfg := &PipelineConfig{
		Steps: []StepConfig{{Name: "build", Command: "go build"}},
	}
	pipelines := cfg.GetPipelines()
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
	if _, ok := pipelines["default"]; !ok {
		t.Error("flat format should produce 'default' pipeline")
	}
}

func TestResolveParameters(t *testing.T) {
	params := []ParameterConfig{
		{Name: "STAGE", Type: "string", Default: "integ-1"},
		{Name: "VERSION", Type: "choice", Options: []string{"v1", "v2"}, Default: "v1"},
		{Name: "DRY_RUN", Type: "boolean", Default: "false"},
	}

	env, err := ResolveParameters(params, map[string]string{"STAGE": "integ-2"})
	if err != nil {
		t.Fatal(err)
	}
	if env["STAGE"] != "integ-2" {
		t.Errorf("STAGE = %q, want integ-2", env["STAGE"])
	}
	if env["VERSION"] != "v1" {
		t.Errorf("VERSION = %q, want v1 (default)", env["VERSION"])
	}
	if env["DRY_RUN"] != "false" {
		t.Errorf("DRY_RUN = %q, want false", env["DRY_RUN"])
	}
}

func TestResolveParameters_InvalidChoice(t *testing.T) {
	params := []ParameterConfig{
		{Name: "V", Type: "choice", Options: []string{"a", "b"}},
	}
	_, err := ResolveParameters(params, map[string]string{"V": "c"})
	if err == nil {
		t.Error("expected error for invalid choice")
	}
}

func TestResolveParameters_InvalidBoolean(t *testing.T) {
	params := []ParameterConfig{
		{Name: "X", Type: "boolean"},
	}
	_, err := ResolveParameters(params, map[string]string{"X": "maybe"})
	if err == nil {
		t.Error("expected error for invalid boolean")
	}
}

func TestValidate_CircularDeps(t *testing.T) {
	cfg := &PipelineConfig{
		Steps: []StepConfig{
			{Name: "a", DependsOn: []string{"b"}},
			{Name: "b", DependsOn: []string{"a"}},
		},
	}
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if contains(e, "circular") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected circular dependency error, got %v", errs)
	}
}

func TestValidate_UnknownDep(t *testing.T) {
	cfg := &PipelineConfig{
		Steps: []StepConfig{
			{Name: "a", DependsOn: []string{"nonexistent"}},
		},
	}
	errs := cfg.Validate()
	if len(errs) == 0 {
		t.Error("expected unknown dep error")
	}
}

func TestValidate_ChoiceNoOptions(t *testing.T) {
	cfg := &PipelineConfig{
		Pipelines: map[string]*Pipeline{
			"test": {
				Parameters: []ParameterConfig{{Name: "X", Type: "choice"}},
				Steps:      []StepConfig{{Name: "a"}},
			},
		},
	}
	errs := cfg.Validate()
	if len(errs) == 0 {
		t.Error("expected choice-no-options error")
	}
}

func TestPostConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := `steps:
  - name: test
    command: ./test.sh
post:
  always:
    - name: cleanup
      command: ./destroy.sh
      timeout: 60m
  on_failure:
    - name: notify
      command: curl $SLACK_WEBHOOK
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Post == nil {
		t.Fatal("post should not be nil")
	}
	if len(cfg.Post.Always) != 1 {
		t.Errorf("expected 1 always step, got %d", len(cfg.Post.Always))
	}
	if len(cfg.Post.OnFailure) != 1 {
		t.Errorf("expected 1 on_failure step, got %d", len(cfg.Post.OnFailure))
	}
	if cfg.Post.Always[0].Timeout != "60m" {
		t.Errorf("cleanup timeout = %q, want 60m", cfg.Post.Always[0].Timeout)
	}
}

func TestServiceContainers(t *testing.T) {
	dir := t.TempDir()
	yaml := `steps:
  - name: e2e
    docker: true
    services:
      - name: postgres
        image: postgres:16
        ports: [5432]
        health:
          cmd: pg_isready
          interval: 10s
          retries: 30
    command: ./test.sh
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	step := cfg.Steps[0]
	if !step.Docker {
		t.Error("docker should be true")
	}
	if len(step.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(step.Services))
	}
	svc := step.Services[0]
	if svc.Name != "postgres" {
		t.Errorf("service name = %q", svc.Name)
	}
	if svc.Health == nil || svc.Health.Retries != 30 {
		t.Error("health check not parsed correctly")
	}
}

func TestConditionalStep(t *testing.T) {
	dir := t.TempDir()
	yaml := `steps:
  - name: deploy-vpc
    when: "$VPC_MODE == 'create'"
    command: ./create-vpc.sh
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Steps[0].When != "$VPC_MODE == 'create'" {
		t.Errorf("when = %q", cfg.Steps[0].When)
	}
}

func TestStepCommands(t *testing.T) {
	step := StepConfig{Commands: []string{"echo hello", "echo world"}}
	if step.GetCommand() != "echo hello && echo world" {
		t.Errorf("GetCommand() = %q", step.GetCommand())
	}
}

func TestParametersParsing(t *testing.T) {
	dir := t.TempDir()
	yaml := `parameters:
  - name: STAGE
    type: string
    default: integ-1
    description: Deployment stage
  - name: SOURCE_VERSION
    type: choice
    options: [ES_7.10, ES_6.8]
    default: ES_7.10
  - name: SKIP_CLEANUP
    type: boolean
    default: "false"
steps:
  - name: deploy
    command: ./deploy.sh
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Parameters) != 3 {
		t.Fatalf("expected 3 parameters, got %d", len(cfg.Parameters))
	}
	if cfg.Parameters[1].Type != "choice" {
		t.Errorf("param[1].Type = %q", cfg.Parameters[1].Type)
	}
	if len(cfg.Parameters[1].Options) != 2 {
		t.Errorf("param[1].Options = %v", cfg.Parameters[1].Options)
	}
}

func TestGateStep(t *testing.T) {
	dir := t.TempDir()
	yaml := `steps:
  - name: build
    command: go build
  - name: test
    command: go test
  - name: all-checks-pass
    type: gate
    depends_on: [build, test]
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Steps[2].Type != "gate" {
		t.Errorf("step type = %q, want gate", cfg.Steps[2].Type)
	}
}

func TestArtifactConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := `steps:
  - name: test
    command: pytest
    artifacts:
      upload:
        - path: /artifacts/report.xml
  - name: aggregate
    depends_on: [test]
    artifacts:
      download:
        - from_step: test
          path: /artifacts/
    command: cat /artifacts/report.xml
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Steps[0].Artifacts.Upload) != 1 {
		t.Error("expected 1 upload artifact")
	}
	if len(cfg.Steps[1].Artifacts.Download) != 1 {
		t.Error("expected 1 download artifact")
	}
	if cfg.Steps[1].Artifacts.Download[0].FromStep != "test" {
		t.Errorf("from_step = %q", cfg.Steps[1].Artifacts.Download[0].FromStep)
	}
}

func TestTriggerStep(t *testing.T) {
	dir := t.TempDir()
	yaml := `steps:
  - name: run-child
    trigger:
      pipeline: k8s-local-test
      parameters:
        SOURCE_VERSION: ES_7.10
      wait: true
      propagate_failure: false
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	tr := cfg.Steps[0].Trigger
	if tr == nil {
		t.Fatal("trigger should not be nil")
	}
	if tr.Pipeline != "k8s-local-test" {
		t.Errorf("pipeline = %q", tr.Pipeline)
	}
	if *tr.PropagateFailure != false {
		t.Error("propagate_failure should be false")
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

func TestMatrixStepConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := `steps:
  - name: gradle-tests
    image: temporalci/ci-base:opensearch-migrations
    matrix:
      index: ["0", "1", "2", "3", "4"]
      fail_fast: false
      max_parallel: 10
    command: ./gradlew allTests -Dtest.striping.total=5 -Dtest.striping.index=${{ matrix.index }}
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	step := cfg.Steps[0]
	if step.Matrix == nil {
		t.Fatal("matrix should not be nil")
	}
	if len(step.Matrix.Dimensions["index"]) != 5 {
		t.Errorf("expected 5 index values, got %d", len(step.Matrix.Dimensions["index"]))
	}
	if step.Matrix.FailFast == nil || *step.Matrix.FailFast != false {
		t.Error("fail_fast should be false")
	}
	if step.Matrix.MaxParallel != 10 {
		t.Errorf("max_parallel = %d, want 10", step.Matrix.MaxParallel)
	}
}

func TestDynamicMatrixConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := `steps:
  - name: generate
    command: echo '["a","b","c"]' > /artifacts/matrix.json
  - name: run
    dynamic_matrix: steps.generate.outputs.matrix
    depends_on: [generate]
    command: echo $MATRIX_VALUE
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Steps[1].DynamicMatrix != "steps.generate.outputs.matrix" {
		t.Errorf("dynamic_matrix = %q", cfg.Steps[1].DynamicMatrix)
	}
}

func TestAllowSkipConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := `steps:
  - name: optional
    command: echo maybe
    allow-skip: true
  - name: gate
    type: gate
    depends_on: [optional]
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Steps[0].AllowSkip {
		t.Error("allow-skip should be true")
	}
}

func TestMatchesChangedPaths_NoPaths(t *testing.T) {
	if !MatchesChangedPaths(nil, []string{"src/main.go"}) {
		t.Error("no paths filter should match everything")
	}
}

func TestMatchesChangedPaths_Exact(t *testing.T) {
	if !MatchesChangedPaths([]string{"src/main.go"}, []string{"src/main.go"}) {
		t.Error("exact match should work")
	}
	if MatchesChangedPaths([]string{"src/main.go"}, []string{"src/other.go"}) {
		t.Error("should not match different file")
	}
}

func TestMatchesChangedPaths_DoubleGlob(t *testing.T) {
	paths := []string{"src/**"}
	if !MatchesChangedPaths(paths, []string{"src/foo/bar.go"}) {
		t.Error("src/** should match src/foo/bar.go")
	}
	if !MatchesChangedPaths(paths, []string{"src/main.go"}) {
		t.Error("src/** should match src/main.go")
	}
	if MatchesChangedPaths(paths, []string{"docs/readme.md"}) {
		t.Error("src/** should not match docs/readme.md")
	}
}

func TestMatchesChangedPaths_Extension(t *testing.T) {
	if !MatchesChangedPaths([]string{"*.go"}, []string{"internal/foo.go"}) {
		t.Error("*.go should match .go files")
	}
	if MatchesChangedPaths([]string{"*.go"}, []string{"readme.md"}) {
		t.Error("*.go should not match .md files")
	}
}

func TestMatchesChangedPaths_DirGlob(t *testing.T) {
	if !MatchesChangedPaths([]string{"docs/*"}, []string{"docs/readme.md"}) {
		t.Error("docs/* should match docs/readme.md")
	}
	if MatchesChangedPaths([]string{"docs/*"}, []string{"docs/sub/file.md"}) {
		t.Error("docs/* should not match subdirectory files")
	}
}

func TestMatchesChangedPaths_DirPrefix(t *testing.T) {
	if !MatchesChangedPaths([]string{"src/"}, []string{"src/foo/bar.go"}) {
		t.Error("src/ should match files under src/")
	}
}

func TestShouldRunWithPaths(t *testing.T) {
	cfg := &PipelineConfig{
		On: &TriggerConfig{
			Push: &PushFilter{
				Branches: []string{"main"},
				Paths:    []string{"src/**", "*.go"},
			},
		},
	}
	if !cfg.ShouldRunWithPaths("push", "main", []string{"src/main.go"}) {
		t.Error("should run for changed src file")
	}
	if cfg.ShouldRunWithPaths("push", "main", []string{"docs/readme.md"}) {
		t.Error("should not run for docs-only change")
	}
	if cfg.ShouldRunWithPaths("push", "main", []string{"go.mod"}) {
		t.Error("go.mod should not match src/** or *.go")
	}
}

func TestShouldRunWithPaths_NoPaths(t *testing.T) {
	cfg := &PipelineConfig{
		On: &TriggerConfig{
			Push: &PushFilter{Branches: []string{"main"}},
		},
	}
	if !cfg.ShouldRunWithPaths("push", "main", []string{"anything.txt"}) {
		t.Error("no path filter should match everything")
	}
}
