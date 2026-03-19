package eval

import (
	"testing"
)

func TestEvaluate_Equality(t *testing.T) {
	ok, err := Evaluate("$MODE == 'create'", map[string]string{"MODE": "create"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestEvaluate_Inequality(t *testing.T) {
	ok, err := Evaluate("$MODE != 'create'", map[string]string{"MODE": "import"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestEvaluate_Contains(t *testing.T) {
	ok, err := Evaluate("$BRANCH contains 'feature'", map[string]string{"BRANCH": "feature/foo"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestEvaluate_StartsWith(t *testing.T) {
	ok, err := Evaluate("$REF startsWith 'refs/tags'", map[string]string{"REF": "refs/tags/v1"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestEvaluate_And(t *testing.T) {
	ok, err := Evaluate("$A == 'x' && $B == 'y'", map[string]string{"A": "x", "B": "y"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestEvaluate_Or(t *testing.T) {
	ok, err := Evaluate("$A == 'x' || $B == 'y'", map[string]string{"A": "z", "B": "y"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestEvaluate_MissingVar(t *testing.T) {
	ok, err := Evaluate("$MISSING == 'x'", map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected false for missing var")
	}
}

func TestEvaluate_Empty(t *testing.T) {
	ok, err := Evaluate("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("empty expression should be true")
	}
}

func TestEvaluate_EventCheck(t *testing.T) {
	ok, err := Evaluate("event == 'push'", map[string]string{"event": "push"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestEvaluate_LabelsContains(t *testing.T) {
	ok, err := Evaluate("$labels contains 'run-eks-tests'", map[string]string{"labels": "run-eks-tests,ci"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestExpandMatrix_Single(t *testing.T) {
	combos := ExpandMatrix(map[string][]string{"index": {"0", "1", "2"}}, nil, nil)
	if len(combos) != 3 {
		t.Fatalf("expected 3 combos, got %d", len(combos))
	}
}

func TestExpandMatrix_TwoDimensions(t *testing.T) {
	combos := ExpandMatrix(map[string][]string{
		"arch": {"x86", "arm"},
		"os":   {"linux", "mac"},
	}, nil, nil)
	if len(combos) != 4 {
		t.Fatalf("expected 4 combos, got %d", len(combos))
	}
}

func TestExpandMatrix_Exclude(t *testing.T) {
	combos := ExpandMatrix(
		map[string][]string{"arch": {"x86", "arm"}, "os": {"linux", "mac"}},
		[]map[string]string{{"arch": "x86", "os": "mac"}},
		nil,
	)
	if len(combos) != 3 {
		t.Fatalf("expected 3 combos, got %d", len(combos))
	}
}

func TestExpandMatrix_Include(t *testing.T) {
	combos := ExpandMatrix(
		map[string][]string{"arch": {"x86", "arm"}, "os": {"linux", "mac"}},
		nil,
		[]map[string]string{{"arch": "riscv", "os": "linux", "extra": "yes"}},
	)
	if len(combos) != 5 {
		t.Fatalf("expected 5 combos, got %d", len(combos))
	}
}

func TestExpandMatrix_EmptyDimension(t *testing.T) {
	combos := ExpandMatrix(map[string][]string{"index": {}}, nil, nil)
	if len(combos) != 0 {
		t.Fatalf("expected 0 combos, got %d", len(combos))
	}
}

func TestMatrixKey_Sorted(t *testing.T) {
	key := MatrixKey(map[string]string{"b": "2", "a": "1"})
	if key != "a=1,b=2" {
		t.Errorf("got %q, want a=1,b=2", key)
	}
}

func TestEvaluate_CompoundCondition(t *testing.T) {
	// opensearch-migrations pattern: event == 'push' || labels contains 'run-eks-tests'
	env := map[string]string{"event": "pull_request", "labels": "run-eks-tests,ci"}
	ok, err := Evaluate("event == 'push' || $labels contains 'run-eks-tests'", env)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true (labels match)")
	}
}

func TestEvaluate_BranchCheck(t *testing.T) {
	ok, err := Evaluate("branch == 'main'", map[string]string{"branch": "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestEvaluate_NegatedCondition(t *testing.T) {
	ok, err := Evaluate("!(event == 'push')", map[string]string{"event": "pull_request"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true for negated condition")
	}
}

func TestEvaluate_BackportBranch(t *testing.T) {
	// delete-backport-branch pattern
	ok, err := Evaluate("$branch startsWith 'backport/'", map[string]string{"branch": "backport/1.x"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true for backport branch")
	}
}
