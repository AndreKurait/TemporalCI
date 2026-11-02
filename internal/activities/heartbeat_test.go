package activities

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestHeartbeatRepeatsAndStops(t *testing.T) {
	var calls atomic.Int32
	repeated := make(chan struct{})
	stop := startHeartbeat(context.Background(), time.Millisecond, func() {
		if calls.Add(1) == 2 {
			close(repeated)
		}
	})
	t.Cleanup(stop)
	select {
	case <-repeated:
	case <-time.After(time.Second):
		t.Fatal("heartbeat did not repeat")
	}
	stop()
	before := calls.Load()
	time.Sleep(10 * time.Millisecond)
	if calls.Load() != before {
		t.Fatal("heartbeat continued after stop returned")
	}
}

func TestHeartbeatStopsOnActivityCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	cancelled := make(chan struct{})
	stop := startHeartbeat(ctx, time.Millisecond, func() {
		if calls.Add(1) == 2 {
			cancel()
			close(cancelled)
		}
	})
	t.Cleanup(stop)
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("heartbeat did not reach cancellation")
	}
	time.Sleep(10 * time.Millisecond)
	if calls.Load() != 2 {
		t.Fatal("heartbeat continued after activity context cancellation")
	}
}

func TestRunStepRecordsActivityHeartbeat(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	var calls atomic.Int32
	env.SetOnActivityHeartbeatListener(func(_ *activity.Info, _ converter.EncodedValues) {
		calls.Add(1)
	})
	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("simulated pod creation failure")
	})
	acts := &Activities{K8sClient: client}
	env.RegisterActivity(acts.RunStep)
	_, err := env.ExecuteActivity(acts.RunStep, RunStepInput{
		Name: "heartbeat-test", Repo: "owner/repo", Image: "unused",
	})
	if err == nil {
		t.Fatal("expected simulated pod creation failure")
	}
	if calls.Load() == 0 {
		t.Fatal("RunStep did not record an activity heartbeat")
	}
}
