package eval

import (
	"fmt"
	"testing"
)

// --- Month 18: Conditional step execution for Jenkins replacement ---

func TestEvaluate_LabelBasedConditional(t *testing.T) {
	// opensearch-migrations pattern: run-eks-tests label
	env := map[string]string{
		"event":  "pull_request",
		"labels": "run-eks-tests,ci",
	}
	ok, err := Evaluate("event == 'push' || $labels contains 'run-eks-tests'", env)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("should match on label")
	}
}

func TestEvaluate_LabelMissing(t *testing.T) {
	env := map[string]string{
		"event":  "pull_request",
		"labels": "ci,docs",
	}
	ok, err := Evaluate("event == 'push' || $labels contains 'run-eks-tests'", env)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("should not match without label")
	}
}

func TestEvaluate_PushOnlyConditional(t *testing.T) {
	ok, err := Evaluate("event == 'push'", map[string]string{"event": "push"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("should match push event")
	}

	ok, err = Evaluate("event == 'push'", map[string]string{"event": "pull_request"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("should not match PR event")
	}
}

func TestEvaluate_BranchConditional(t *testing.T) {
	ok, err := Evaluate("branch == 'main'", map[string]string{"branch": "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("should match main branch")
	}
}

func TestEvaluate_BackportBranchStartsWith(t *testing.T) {
	ok, err := Evaluate("$branch startsWith 'backport/'", map[string]string{"branch": "backport/1.x"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("should match backport branch")
	}

	ok, err = Evaluate("$branch startsWith 'backport/'", map[string]string{"branch": "feature/x"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("should not match feature branch")
	}
}

func TestEvaluate_ComplexJenkinsConditional(t *testing.T) {
	// Jenkins replacement: run if push to main OR has specific label
	env := map[string]string{
		"event":  "pull_request",
		"branch": "feature/x",
		"labels": "run-eks-tests",
	}
	ok, err := Evaluate("(event == 'push' && branch == 'main') || $labels contains 'run-eks-tests'", env)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("should match on label even though not push to main")
	}
}

func TestEvaluate_NegatedEvent(t *testing.T) {
	ok, err := Evaluate("!(event == 'schedule')", map[string]string{"event": "push"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("should be true for non-schedule event")
	}
}

func TestExpandMatrix_JenkinsReplacement(t *testing.T) {
	// Jenkins jobs: 3 different test suites
	combos := ExpandMatrix(map[string][]string{
		"suite": {"full-es68-e2e-aws-test", "elasticsearch-5x-k8s-local-test", "eks-integ-test"},
	}, nil, nil)
	if len(combos) != 3 {
		t.Fatalf("expected 3 combos, got %d", len(combos))
	}
}

func TestExpandMatrix_OpensearchMigrations30Way(t *testing.T) {
	// Full 30-way Gradle striping
	indices := make([]string, 30)
	for i := range indices {
		indices[i] = fmt.Sprintf("%d", i)
	}
	combos := ExpandMatrix(map[string][]string{"index": indices}, nil, nil)
	if len(combos) != 30 {
		t.Fatalf("expected 30 combos, got %d", len(combos))
	}
}
