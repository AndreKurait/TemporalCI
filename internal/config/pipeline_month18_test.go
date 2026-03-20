package config

import (
	"testing"
)

// --- Month 18: Full opensearch-migrations pipeline, conditional steps, Jenkins replacement ---

func TestLoadPipelineConfig_FullOpensearchMigrations(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "ci", `on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
steps:
  - name: gradle-extended-check
    image: temporalci/ci-base:opensearch-migrations
    matrix:
      index: ["0", "1"]
    command: ./gradlew spotlessCheck publishToMavenLocal
    timeout: 15m

  - name: python-lint
    image: temporalci/ci-base:opensearch-migrations
    command: flake8 .
    timeout: 5m

  - name: gradle-tests
    image: temporalci/ci-base:opensearch-migrations
    matrix:
      index: ["0","1","2","3","4","5","6","7","8","9","10","11","12","13","14","15","16","17","18","19","20","21","22","23","24","25","26","27","28","29"]
    command: ./gradlew allTests -Dtest.striping.total=30 -Dtest.striping.index=${{ matrix.index }}
    docker: true
    timeout: 30m
    post:
      - "codecov --flag gradle"

  - name: python-tests
    image: temporalci/ci-base:opensearch-migrations
    matrix:
      project: [console_link, metadata_migration, fetch_migration]
    command: cd ${{ matrix.project }} && pipenv run pytest
    timeout: 15m

  - name: python-e2e-tests
    image: temporalci/ci-base:opensearch-migrations
    docker: true
    command: docker-compose up -d && pytest e2e/
    timeout: 30m

  - name: node-tests
    image: temporalci/ci-base:opensearch-migrations
    matrix:
      project: [opensearch-dashboards-plugin, capture-proxy-ui, traffic-comparator-ui]
    command: cd ${{ matrix.project }} && npm test
    timeout: 10m

  - name: link-checker
    image: temporalci/ci-base:opensearch-migrations
    command: lychee --no-progress .
    timeout: 5m

  - name: all-ci-checks-pass
    type: gate
    depends_on: [gradle-extended-check, python-lint, gradle-tests, python-tests, python-e2e-tests, node-tests, link-checker]
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Pipelines["ci"]
	if len(p.Steps) != 8 {
		t.Fatalf("expected 8 steps, got %d", len(p.Steps))
	}

	// Validate no errors
	errs := cfg.Validate()
	if len(errs) != 0 {
		t.Errorf("unexpected validation errors: %v", errs)
	}

	// Check gate step
	gate := p.Steps[7]
	if gate.Type != "gate" {
		t.Errorf("last step type = %q, want gate", gate.Type)
	}
	if len(gate.DependsOn) != 7 {
		t.Errorf("gate depends_on = %d, want 7", len(gate.DependsOn))
	}

	// Check 30-way matrix
	gradleTests := p.Steps[2]
	if gradleTests.Matrix == nil {
		t.Fatal("gradle-tests should have matrix")
	}
	if len(gradleTests.Matrix.Dimensions["index"]) != 30 {
		t.Errorf("gradle matrix = %d, want 30", len(gradleTests.Matrix.Dimensions["index"]))
	}
	if !gradleTests.Docker {
		t.Error("gradle-tests should have docker=true")
	}
	if len(gradleTests.Post) != 1 {
		t.Errorf("gradle-tests post = %v", gradleTests.Post)
	}
}

func TestLoadPipelineConfig_ConditionalSteps(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "jenkins", `on:
  pull_request:
    labels: [run-eks-tests]
steps:
  - name: eks-test
    if: "event == 'push' || labels contains 'run-eks-tests'"
    command: ./run-eks-tests.sh
    privileged: true
    timeout: 60m
  - name: optional-step
    if: "branch == 'main'"
    command: echo "only on main"
    allow-skip: true
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Pipelines["jenkins"]
	if p.Steps[0].GetCondition() != "event == 'push' || labels contains 'run-eks-tests'" {
		t.Errorf("condition = %q", p.Steps[0].GetCondition())
	}
	if !p.Steps[0].Privileged {
		t.Error("eks-test should be privileged")
	}
	if !p.Steps[1].AllowSkip {
		t.Error("optional-step should have allow-skip")
	}
}

func TestLoadPipelineConfig_UtilityWorkflows(t *testing.T) {
	dir := t.TempDir()

	// add-untriaged
	writePipeline(t, dir, "add-untriaged", `on:
  issues:
    types: [opened, reopened, transferred]
steps:
  - name: add-label
    command: echo "add untriaged label"
`)

	// delete-backport-branch
	writePipeline(t, dir, "delete-backport-branch", `on:
  pull_request:
    branches: [main]
steps:
  - name: delete-branch
    if: "$branch startsWith 'backport/'"
    command: echo "delete branch"
`)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Pipelines) != 2 {
		t.Fatalf("expected 2 pipelines, got %d", len(cfg.Pipelines))
	}

	untriaged := cfg.Pipelines["add-untriaged"]
	if untriaged.On.Issues == nil {
		t.Error("add-untriaged should have issues trigger")
	}
	if len(untriaged.On.Issues.Types) != 3 {
		t.Errorf("issue types = %v", untriaged.On.Issues.Types)
	}

	backport := cfg.Pipelines["delete-backport-branch"]
	if backport.Steps[0].GetCondition() != "$branch startsWith 'backport/'" {
		t.Errorf("condition = %q", backport.Steps[0].GetCondition())
	}
}

func TestLoadPipelineConfig_ReleaseWorkflow(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "release", `on:
  release:
    types: [published]
steps:
  - name: approval
    type: gate
  - name: build
    docker: true
    privileged: true
    command: docker buildx build --platform linux/amd64,linux/arm64 --push .
    timeout: 30m
  - name: sbom
    command: syft packages . -o spdx-json > sbom.json
    depends_on: [build]
  - name: publish
    command: ./gradlew publishMavenJavaPublicationToMavenRepository
    depends_on: [build]
    aws_role:
      arn: arn:aws:iam::123:role/publish
  - name: create-release
    command: echo "create github release"
    depends_on: [build, sbom, publish]
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Pipelines["release"]
	if p.On.Release == nil {
		t.Error("expected release trigger")
	}
	if len(p.Steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(p.Steps))
	}
	errs := cfg.Validate()
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}
