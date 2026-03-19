package workflows

import (
	"testing"

	"github.com/AndreKurait/TemporalCI/internal/activities"
)

func TestBuildStepSecrets(t *testing.T) {
	resolved := map[string]string{"A": "1", "B": "2", "C": "3"}
	step := activities.StepConfig{Secrets: []string{"A", "C"}}
	got := buildStepSecrets(step, resolved)
	if got["A"] != "1" || got["C"] != "3" {
		t.Errorf("got %v", got)
	}
	if _, ok := got["B"]; ok {
		t.Error("should not include unrequested secret B")
	}
}

func TestBuildStepSecrets_Empty(t *testing.T) {
	got := buildStepSecrets(activities.StepConfig{}, map[string]string{"A": "1"})
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestMergeEnv(t *testing.T) {
	got := mergeEnv(
		map[string]string{"A": "1", "B": "2"},
		map[string]string{"B": "override", "C": "3"},
	)
	if got["A"] != "1" || got["B"] != "override" || got["C"] != "3" {
		t.Errorf("got %v", got)
	}
}

func TestFlattenOutputs(t *testing.T) {
	all := map[string]map[string]string{
		"step1": {"X": "1"},
		"step2": {"Y": "2"},
	}
	got := flattenOutputs(all)
	if got["X"] != "1" || got["Y"] != "2" {
		t.Errorf("got %v", got)
	}
}

func TestResolveDynamicMatrix_JSONObject(t *testing.T) {
	outputs := map[string]map[string]string{
		"gen": {"matrix": `{"arch":["x86","arm"],"os":["linux","mac"]}`},
	}
	got := resolveDynamicMatrix("gen", outputs)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if len(got["arch"]) != 2 || len(got["os"]) != 2 {
		t.Errorf("got %v", got)
	}
}

func TestResolveDynamicMatrix_JSONArray(t *testing.T) {
	outputs := map[string]map[string]string{
		"gen": {"matrix": `["a","b","c"]`},
	}
	got := resolveDynamicMatrix("gen", outputs)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if len(got["value"]) != 3 {
		t.Errorf("got %v", got)
	}
}

func TestResolveDynamicMatrix_StepsDotSyntax(t *testing.T) {
	outputs := map[string]map[string]string{
		"gen": {"matrix": `["x","y"]`},
	}
	got := resolveDynamicMatrix("steps.gen.outputs.matrix", outputs)
	if got == nil || len(got["value"]) != 2 {
		t.Errorf("got %v", got)
	}
}

func TestResolveDynamicMatrix_MissingStep(t *testing.T) {
	got := resolveDynamicMatrix("nonexistent", map[string]map[string]string{})
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestResolveDynamicMatrix_InvalidJSON(t *testing.T) {
	outputs := map[string]map[string]string{
		"gen": {"matrix": `not json`},
	}
	got := resolveDynamicMatrix("gen", outputs)
	if got != nil {
		t.Errorf("expected nil for invalid JSON, got %v", got)
	}
}
