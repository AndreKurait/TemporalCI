package activities

import (
	"context"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestPrivilegedExecutionRequiresExplicitRepositoryAllowance(t *testing.T) {
	for _, tc := range []struct {
		name       string
		allowed    map[string]bool
		privileged bool
		docker     bool
	}{
		{name: "privileged with unset allowlist", privileged: true},
		{name: "docker with unset allowlist", docker: true},
		{name: "other repository", allowed: map[string]bool{"other/repo": true}, privileged: true},
		{name: "explicitly disabled repository", allowed: map[string]bool{"owner/repo": false}, docker: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			acts := &Activities{K8sClient: client, PrivilegedRepos: tc.allowed}
			_, err := acts.runStepK8s(context.Background(), RunStepInput{
				Repo: "owner/repo", Privileged: tc.privileged, Docker: tc.docker,
			})
			if err == nil || !strings.Contains(err.Error(), "not allowed to use privileged mode") {
				t.Fatalf("expected privileged execution to be rejected, got %v", err)
			}
			if actions := client.Actions(); len(actions) != 0 {
				t.Fatalf("denied execution performed Kubernetes actions: %v", actions)
			}
		})
	}
}
