package workflows

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/AndreKurait/TemporalCI/internal/activities"
)

func testRunStepsWorkflow(ctx workflow.Context, steps []activities.StepConfig) (CIPipelineResult, error) {
	var acts *activities.Activities
	results, status, _ := runSteps(ctx, acts, steps, CIPipelineInput{
		Repo: "owner/repo", Event: "push", PipelineName: "ci",
	}, "/unused", nil, map[string]string{"TEST_PARAMETER": "preserved"})
	return CIPipelineResult{Status: status, Steps: results}, nil
}

func TestMatrixFailureBlocksDependentsRegardlessOfFailFast(t *testing.T) {
	for _, tc := range []struct {
		name           string
		failFast, fail bool
		matrixCalls    int
	}{
		{"continue matrix after failure", false, true, 3},
		{"stop matrix after failure", true, true, 1},
		{"successful matrix allows dependent", false, false, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			env.RegisterWorkflow(MatrixChild)
			var acts *activities.Activities
			var mu sync.Mutex
			matrixCalls, deployCalls, independentCalls := 0, 0, 0
			env.OnActivity(acts.RunStep, mock.Anything, mock.Anything).Return(
				func(_ context.Context, input activities.RunStepInput) (activities.RunStepResult, error) {
					mu.Lock()
					defer mu.Unlock()
					if len(input.MatrixVars) > 0 {
						matrixCalls++
						if input.Command != "echo first && echo second" {
							t.Errorf("matrix commands not forwarded: %q", input.Command)
						}
						if input.Secrets["TEST_PARAMETER"] != "preserved" {
							t.Error("matrix lost pipeline parameter")
						}
						if tc.fail && input.MatrixVars["os"] == "a" {
							return activities.RunStepResult{}, errors.New("repository not allowed to use privileged mode")
						}
					} else if input.Name == "deploy" {
						deployCalls++
					} else if input.Name == "independent" {
						independentCalls++
					}
					return activities.RunStepResult{}, nil
				},
			)
			env.ExecuteWorkflow(testRunStepsWorkflow, []activities.StepConfig{
				{Name: "matrix", Commands: []string{"echo first", "echo second"},
					Matrix: &activities.MatrixConfig{
						Dimensions: map[string][]string{"os": {"a", "b", "c"}},
						FailFast:   tc.failFast, MaxParallel: 1,
					}},
				{Name: "deploy", DependsOn: []string{"matrix"}, Command: "echo deploy"},
				{Name: "independent", Command: "echo independent"},
			})
			if err := env.GetWorkflowError(); err != nil {
				t.Fatal(err)
			}
			var result CIPipelineResult
			if err := env.GetWorkflowResult(&result); err != nil {
				t.Fatal(err)
			}
			wantStatus, wantDeploy := "passed", 1
			if tc.fail {
				wantStatus, wantDeploy = "failed", 0
			}
			if result.Status != wantStatus || matrixCalls != tc.matrixCalls || deployCalls != wantDeploy || independentCalls != 1 {
				t.Fatalf("result=%+v calls matrix=%d deploy=%d independent=%d", result, matrixCalls, deployCalls, independentCalls)
			}
			env.AssertExpectations(t)
		})
	}
}

func TestMatrixChildWorkflowErrorIsReportedAsFailure(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(MatrixChild)
	env.OnWorkflow(MatrixChild, mock.Anything, mock.Anything).
		Return(activities.MatrixChildResult{}, errors.New("child workflow failed")).Once()
	env.ExecuteWorkflow(testRunStepsWorkflow, []activities.StepConfig{
		{Name: "matrix", Matrix: &activities.MatrixConfig{Dimensions: map[string][]string{"os": {"linux"}}}},
		{Name: "deploy", DependsOn: []string{"matrix"}},
	})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result CIPipelineResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" || len(result.Steps) != 2 || result.Steps[0].Status != "failed" || result.Steps[1].Status != "skipped" {
		t.Fatalf("child error was lost: %+v", result)
	}
	env.AssertExpectations(t)
}

func TestUnmatchedPipelineSkipsExecutionAndSecrets(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var acts *activities.Activities
	env.OnActivity(acts.CloneRepo, mock.Anything, mock.MatchedBy(func(input activities.CloneInput) bool {
		return input.Event == "push" && len(input.ChangedFiles) == 1 && input.ChangedFiles[0] == "main.go"
	})).Return(activities.CloneResult{Dir: "/unused", Skipped: true}, nil).Once()
	env.OnActivity(acts.Cleanup, mock.Anything, "/unused").Return(nil).Once()
	env.ExecuteWorkflow(CIPipeline, CIPipelineInput{Event: "push", Ref: "main", ChangedFiles: []string{"main.go"}})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result CIPipelineResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "skipped" || len(result.Steps) != 0 {
		t.Fatalf("unmatched pipeline executed: %+v", result)
	}
	env.AssertExpectations(t)
}

func testDispatchWorkflow(ctx workflow.Context, names []string) (CIPipelineResult, error) {
	return dispatchPipelines(ctx, CIPipelineInput{Event: "push"}, activities.CloneResult{Pipelines: names})
}

func TestDispatchIncludesMatchingDefaultPipeline(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(CIPipeline)
	for _, name := range []string{"ci", "default"} {
		env.OnWorkflow(CIPipeline, mock.Anything, CIPipelineInput{Event: "push", PipelineName: name}).
			Return(CIPipelineResult{Status: "passed", Steps: []activities.StepResult{{Name: name, Status: "passed"}}}, nil).Once()
	}
	env.ExecuteWorkflow(testDispatchWorkflow, []string{"ci", "default"})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result CIPipelineResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "passed" || len(result.Steps) != 2 {
		t.Fatalf("matching default pipeline was not dispatched: %+v", result)
	}
	env.AssertExpectations(t)
}

func TestDispatchCancelledChildFailsParent(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(CIPipeline)
	env.OnWorkflow(CIPipeline, mock.Anything, CIPipelineInput{Event: "push", PipelineName: "ci"}).
		Return(CIPipelineResult{Status: "passed", Steps: []activities.StepResult{{Name: "test", Status: "passed"}}}, nil).Once()
	env.OnWorkflow(CIPipeline, mock.Anything, CIPipelineInput{Event: "push", PipelineName: "deploy"}).
		Return(CIPipelineResult{Status: "cancelled", Steps: []activities.StepResult{{Name: "publish", Status: "cancelled"}}}, nil).Once()

	env.ExecuteWorkflow(testDispatchWorkflow, []string{"ci", "deploy"})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result CIPipelineResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" || len(result.Steps) != 2 ||
		result.Steps[0].Name != "test" || result.Steps[0].Status != "passed" ||
		result.Steps[1].Name != "publish" || result.Steps[1].Status != "cancelled" {
		t.Fatalf("cancelled child was lost or reported as success: %+v", result)
	}
	env.AssertExpectations(t)
}

func TestTriggeredCancelledChildFailsParentAndBlocksDependents(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(CIPipeline)
	env.OnWorkflow(CIPipeline, mock.Anything, CIPipelineInput{
		Event: "pipeline", Repo: "owner/repo", PipelineName: "child",
	}).Return(CIPipelineResult{Status: "cancelled"}, nil).Once()
	var acts *activities.Activities
	env.OnActivity(acts.RunStep, mock.Anything, mock.MatchedBy(func(input activities.RunStepInput) bool {
		return input.Name == "independent"
	})).Return(activities.RunStepResult{}, nil).Once()

	env.ExecuteWorkflow(testRunStepsWorkflow, []activities.StepConfig{
		{Name: "trigger-child", Trigger: &activities.TriggerStep{
			Pipeline: "child", Wait: true, PropagateFailure: true,
		}},
		{Name: "deploy", DependsOn: []string{"trigger-child"}, Command: "echo deploy"},
		{Name: "independent", Command: "echo independent"},
	})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result CIPipelineResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" || len(result.Steps) != 3 ||
		result.Steps[0].Status != "cancelled" || result.Steps[0].Outputs["CHILD_RESULT"] != "cancelled" ||
		result.Steps[1].Status != "skipped" || result.Steps[2].Status != "passed" {
		t.Fatalf("cancelled child did not fail parent and block dependents: %+v", result)
	}
	env.AssertExpectations(t)
}
