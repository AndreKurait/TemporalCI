package activities

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"go.temporal.io/sdk/activity"
)

// ClusterInput defines the input for provisioning an EKS cluster.
type ClusterInput struct {
	Name        string   `json:"name"`
	Region      string   `json:"region"`
	SubnetIDs   []string `json:"subnetIDs"`
	RoleARN     string   `json:"roleARN"`
	NodeRoleARN string   `json:"nodeRoleARN"`
}

// ClusterResult defines the output of a provisioned cluster.
type ClusterResult struct {
	Name                 string `json:"name"`
	Endpoint             string `json:"endpoint"`
	CertificateAuthority string `json:"certificateAuthority"`
	Kubeconfig           string `json:"kubeconfig"`
}

// DestroyClusterInput defines the input for destroying a cluster.
type DestroyClusterInput struct {
	Name   string `json:"name"`
	Region string `json:"region"`
}

// HelmTestInput defines the input for running helm install + test.
type HelmTestInput struct {
	Kubeconfig  string            `json:"kubeconfig"`
	ChartPath   string            `json:"chartPath"`
	ReleaseName string            `json:"releaseName"`
	Namespace   string            `json:"namespace"`
	Values      map[string]string `json:"values,omitempty"`
	Timeout     string            `json:"timeout"`
}

// HelmTestResult defines the output of a helm test run.
type HelmTestResult struct {
	InstallOutput string `json:"installOutput"`
	TestOutput    string `json:"testOutput"`
	Passed        bool   `json:"passed"`
}

// ProvisionCluster creates an EKS Auto Mode cluster and waits for it to be ACTIVE.
func (a *Activities) ProvisionCluster(ctx context.Context, input ClusterInput) (ClusterResult, error) {
	log := a.logger(ctx).With("cluster", input.Name)
	log.Info("provisioning EKS cluster")

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(input.Region))
	if err != nil {
		return ClusterResult{}, fmt.Errorf("aws config: %w", err)
	}
	client := eks.NewFromConfig(cfg)

	_, err = client.CreateCluster(ctx, &eks.CreateClusterInput{
		Name:    &input.Name,
		RoleArn: &input.RoleARN,
		ResourcesVpcConfig: &types.VpcConfigRequest{
			SubnetIds: input.SubnetIDs,
		},
		AccessConfig: &types.CreateAccessConfigRequest{
			AuthenticationMode: types.AuthenticationModeApiAndConfigMap,
		},
	})
	if err != nil {
		return ClusterResult{}, fmt.Errorf("create cluster: %w", err)
	}

	// Poll until ACTIVE
	for {
		activity.RecordHeartbeat(ctx, "waiting for cluster")
		time.Sleep(30 * time.Second)

		desc, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: &input.Name})
		if err != nil {
			return ClusterResult{}, fmt.Errorf("describe cluster: %w", err)
		}
		log.Info("cluster status", "status", desc.Cluster.Status)
		if desc.Cluster.Status == types.ClusterStatusActive {
			kubeconfig := buildKubeconfig(input.Name, *desc.Cluster.Endpoint,
				*desc.Cluster.CertificateAuthority.Data, input.Region)
			return ClusterResult{
				Name:                 input.Name,
				Endpoint:             *desc.Cluster.Endpoint,
				CertificateAuthority: *desc.Cluster.CertificateAuthority.Data,
				Kubeconfig:           kubeconfig,
			}, nil
		}
		if desc.Cluster.Status == types.ClusterStatusFailed {
			return ClusterResult{}, fmt.Errorf("cluster creation failed")
		}
	}
}

// DestroyCluster deletes an EKS cluster.
func (a *Activities) DestroyCluster(ctx context.Context, input DestroyClusterInput) error {
	log := a.logger(ctx).With("cluster", input.Name)
	log.Info("destroying EKS cluster")

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(input.Region))
	if err != nil {
		return fmt.Errorf("aws config: %w", err)
	}
	client := eks.NewFromConfig(cfg)

	_, err = client.DeleteCluster(ctx, &eks.DeleteClusterInput{Name: &input.Name})
	if err != nil {
		return fmt.Errorf("delete cluster: %w", err)
	}

	// Poll until gone
	for {
		activity.RecordHeartbeat(ctx, "waiting for cluster deletion")
		time.Sleep(15 * time.Second)
		_, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: &input.Name})
		if err != nil {
			log.Info("cluster deleted")
			return nil // cluster gone
		}
	}
}

// RunHelmTest runs helm install + helm test against a cluster using the provided kubeconfig.
func (a *Activities) RunHelmTest(ctx context.Context, input HelmTestInput) (HelmTestResult, error) {
	log := a.logger(ctx).With("release", input.ReleaseName, "chart", input.ChartPath)
	log.Info("running helm test")

	// Write kubeconfig to temp file
	kubeconfigFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return HelmTestResult{}, err
	}
	defer os.Remove(kubeconfigFile.Name())
	if _, err := kubeconfigFile.WriteString(input.Kubeconfig); err != nil {
		return HelmTestResult{}, err
	}
	kubeconfigFile.Close()

	timeout := input.Timeout
	if timeout == "" {
		timeout = "5m"
	}
	ns := input.Namespace
	if ns == "" {
		ns = "test"
	}

	// Build helm install args
	installArgs := []string{"install", input.ReleaseName, input.ChartPath,
		"--kubeconfig", kubeconfigFile.Name(),
		"--namespace", ns, "--create-namespace",
		"--wait", "--timeout", timeout}
	for k, v := range input.Values {
		installArgs = append(installArgs, "--set", fmt.Sprintf("%s=%s", k, v))
	}

	// helm install
	installOut, installErr := exec.CommandContext(ctx, "helm", installArgs...).CombinedOutput()
	result := HelmTestResult{InstallOutput: string(installOut)}
	if installErr != nil {
		result.Passed = false
		// Cleanup
		exec.CommandContext(ctx, "helm", "uninstall", input.ReleaseName,
			"--kubeconfig", kubeconfigFile.Name(), "--namespace", ns).Run()
		return result, nil // don't error — report the failure
	}

	// helm test
	testOut, testErr := exec.CommandContext(ctx, "helm", "test", input.ReleaseName,
		"--kubeconfig", kubeconfigFile.Name(),
		"--namespace", ns, "--timeout", timeout).CombinedOutput()
	result.TestOutput = string(testOut)
	result.Passed = testErr == nil

	// Cleanup
	exec.CommandContext(ctx, "helm", "uninstall", input.ReleaseName,
		"--kubeconfig", kubeconfigFile.Name(), "--namespace", ns).Run()

	return result, nil
}

func buildKubeconfig(name, endpoint, caData, region string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %s
    certificate-authority-data: %s
  name: %s
contexts:
- context:
    cluster: %s
    user: %s
  name: %s
current-context: %s
users:
- name: %s
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args: ["--region", "%s", "eks", "get-token", "--cluster-name", "%s", "--output", "json"]
`, endpoint, base64.StdEncoding.EncodeToString([]byte(caData)),
		name, name, name, name, name, name, region, name)
}
