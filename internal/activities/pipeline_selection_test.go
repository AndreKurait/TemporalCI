package activities

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPipelineSelectionFiltersAutomaticDispatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".temporalci"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{
		"ci":              "on:\n  push: {}\n  pull_request: {}\nsteps:\n  - name: test\n    command: echo test\n",
		"deploy-site":     "on:\n  push:\n    branches: [main]\n    paths: ['site/**']\nsteps:\n  - name: deploy\n    command: echo deploy\n",
		"terraform-apply": "on:\n  webhook: {}\nparameters:\n  - name: ACTION\n    type: choice\n    default: plan\n    options: [plan, apply]\nsteps:\n  - name: terraform\n    command: echo terraform\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, ".temporalci", name+".yaml"), []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name    string
		input   CloneInput
		want    []string
		skipped bool
		wantErr bool
	}{
		{"code push", CloneInput{Event: "push", Ref: "refs/heads/main", ChangedFiles: []string{"main.go"}}, []string{"ci"}, false, false},
		{"site push", CloneInput{Event: "push", Ref: "refs/heads/main", ChangedFiles: []string{"site/index.md"}}, []string{"ci", "deploy-site"}, false, false},
		{"feature push", CloneInput{Event: "push", Ref: "refs/heads/feature", ChangedFiles: []string{"site/index.md"}}, []string{"ci"}, false, false},
		{"pull request", CloneInput{Event: "pull_request", Ref: "feature"}, []string{"ci"}, false, false},
		{"named automatic mismatch", CloneInput{Event: "push", Ref: "main", PipelineName: "terraform-apply"}, nil, true, false},
		{"manual selected terraform", CloneInput{Event: "manual", Ref: "main", PipelineName: "terraform-apply"}, []string{"terraform-apply"}, false, false},
		{"manual selected ci", CloneInput{Event: "manual", Ref: "main", PipelineName: "ci"}, []string{"ci"}, false, false},
		{"webhook", CloneInput{Event: "webhook", Ref: "main"}, []string{"terraform-apply"}, false, false},
		{"unknown name", CloneInput{Event: "manual", Ref: "main", PipelineName: "typo"}, nil, false, true},
		{"no matches", CloneInput{Event: "schedule", Ref: "main"}, nil, true, false},
		{"invalid parameter", CloneInput{Event: "manual", PipelineName: "terraform-apply", Parameters: map[string]string{"ACTION": "oops"}}, nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := loadSelectedPipeline(dir, tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if !reflect.DeepEqual(result.Pipelines, tc.want) || result.Skipped != tc.skipped {
				t.Fatalf("selection = %+v; want %v, skipped %v", result, tc.want, tc.skipped)
			}
			if len(tc.want) == 1 {
				if len(result.Steps) != 1 || result.PipelineName != tc.want[0] {
					t.Fatalf("selected pipeline missing steps/name: %+v", result)
				}
				if result.PipelineName == "terraform-apply" && result.Parameters["ACTION"] != "plan" {
					t.Fatal("missing safe default ACTION=plan")
				}
			} else if len(result.Steps) != 0 {
				t.Fatal("unselected pipeline steps must not be returned")
			}
		})
	}
}
