package activities

import (
	"fmt"
	"strings"
	"testing"
)

// --- Month 16: Cluster provisioning, kubeconfig, helm test inputs ---

func TestBuildKubeconfig(t *testing.T) {
	kc := buildKubeconfig("test-cluster", "https://eks.example.com", "Y2VydA==", "us-west-2")
	if !strings.Contains(kc, "test-cluster") {
		t.Error("should contain cluster name")
	}
	if !strings.Contains(kc, "https://eks.example.com") {
		t.Error("should contain endpoint")
	}
	if !strings.Contains(kc, "Y2VydA==") {
		t.Error("should contain CA data")
	}
	if !strings.Contains(kc, "us-west-2") {
		t.Error("should contain region")
	}
	if !strings.Contains(kc, "apiVersion: v1") {
		t.Error("should be valid kubeconfig format")
	}
}

func TestClusterInput_Fields(t *testing.T) {
	input := ClusterInput{
		Name:        "ci-pool-1",
		Region:      "us-west-2",
		SubnetIDs:   []string{"subnet-1", "subnet-2"},
		RoleARN:     "arn:aws:iam::123:role/cluster",
		NodeRoleARN: "arn:aws:iam::123:role/node",
	}
	if len(input.SubnetIDs) != 2 {
		t.Errorf("subnets = %v", input.SubnetIDs)
	}
}

func TestHelmTestInput_Defaults(t *testing.T) {
	input := HelmTestInput{
		ChartPath:   "./charts/myapp",
		ReleaseName: "myapp-test",
		Values:      map[string]string{"image.tag": "latest"},
	}
	if input.Namespace != "" {
		t.Error("namespace should default to empty (set to 'test' in activity)")
	}
	if input.Timeout != "" {
		t.Error("timeout should default to empty (set to '5m' in activity)")
	}
}

func TestIsAlreadyExists(t *testing.T) {
	if isAlreadyExists(nil) {
		t.Error("nil should not be already exists")
	}
	if !isAlreadyExists(fmt.Errorf("ResourceInUseException: cluster exists")) {
		t.Error("should detect ResourceInUseException")
	}
	if isAlreadyExists(fmt.Errorf("some other error")) {
		t.Error("should not match other errors")
	}
}
