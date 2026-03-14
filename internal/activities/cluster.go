package activities

import (
	"context"
	"fmt"
	"os/exec"

	"go.temporal.io/sdk/activity"
)

// ProvisionCluster creates a new EKS cluster for CI testing.
func (a *Activities) ProvisionCluster(ctx context.Context, input ClusterLeaseInput) (ClusterLeaseResult, error) {
	logger := activity.GetLogger(ctx)
	info := activity.GetInfo(ctx)
	clusterName := fmt.Sprintf("ci-pool-%s", info.ActivityID)
	logger.Info("Provisioning cluster", "name", clusterName, "pool", input.Pool)

	// Create EKS cluster via AWS CLI
	cmd := exec.CommandContext(ctx, "aws", "eks", "create-cluster",
		"--name", clusterName,
		"--region", a.AWSRegion,
		"--role-arn", a.ClusterRoleARN,
		"--resources-vpc-config", fmt.Sprintf("subnetIds=%s", a.SubnetIDs),
		"--compute-config", "enabled=true,nodePools=general-purpose",
		"--kubernetes-network-config", "elasticLoadBalancing={enabled=true}",
		"--storage-config", "blockStorage={enabled=true}",
		"--access-config", "authenticationMode=API_AND_CONFIG_MAP",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ClusterLeaseResult{}, fmt.Errorf("create cluster: %s: %s", err, out)
	}

	// Wait for cluster to be active
	waitCmd := exec.CommandContext(ctx, "aws", "eks", "wait", "cluster-active",
		"--name", clusterName, "--region", a.AWSRegion)
	if out, err := waitCmd.CombinedOutput(); err != nil {
		return ClusterLeaseResult{}, fmt.Errorf("wait cluster: %s: %s", err, out)
	}

	// Get cluster details
	descCmd := exec.CommandContext(ctx, "aws", "eks", "describe-cluster",
		"--name", clusterName, "--region", a.AWSRegion,
		"--query", "cluster.[endpoint,certificateAuthority.data]",
		"--output", "text")
	descOut, err := descCmd.CombinedOutput()
	if err != nil {
		return ClusterLeaseResult{}, fmt.Errorf("describe cluster: %s: %s", err, descOut)
	}

	logger.Info("Cluster provisioned", "name", clusterName)
	return ClusterLeaseResult{
		ClusterName: clusterName,
		Region:      a.AWSRegion,
	}, nil
}

// DestroyCluster deletes an EKS cluster.
func (a *Activities) DestroyCluster(ctx context.Context, clusterName string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Destroying cluster", "name", clusterName)

	cmd := exec.CommandContext(ctx, "aws", "eks", "delete-cluster",
		"--name", clusterName, "--region", a.AWSRegion)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("delete cluster: %s: %s", err, out)
	}
	return nil
}

// LeaseCluster leases a cluster from the pool (or provisions a new one).
func (a *Activities) LeaseCluster(ctx context.Context, input ClusterLeaseInput) (ClusterLeaseResult, error) {
	// For now, provision a new cluster directly
	// In production, this would signal the ClusterPool workflow
	return a.ProvisionCluster(ctx, input)
}

// ReleaseCluster releases a cluster back to the pool.
func (a *Activities) ReleaseCluster(ctx context.Context, input ClusterReleaseInput) error {
	if input.Destroy {
		return a.DestroyCluster(ctx, input.ClusterName)
	}
	// In production, signal the ClusterPool workflow to return to warm pool
	return nil
}

// HelmDeploy installs a Helm chart on a cluster.
func (a *Activities) HelmDeploy(ctx context.Context, input HelmDeployInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Deploying Helm chart", "cluster", input.ClusterName, "chart", input.Chart)

	// Update kubeconfig for the target cluster
	kubecfg := exec.CommandContext(ctx, "aws", "eks", "update-kubeconfig",
		"--name", input.ClusterName, "--region", a.AWSRegion,
		"--kubeconfig", "/tmp/kubeconfig-"+input.ClusterName)
	if out, err := kubecfg.CombinedOutput(); err != nil {
		return fmt.Errorf("kubeconfig: %s: %s", err, out)
	}

	args := []string{
		"upgrade", "--install", input.ReleaseName, input.Chart,
		"--namespace", input.Namespace, "--create-namespace",
		"--kubeconfig", "/tmp/kubeconfig-" + input.ClusterName,
		"--wait", "--timeout", "5m",
	}
	if input.Values != "" {
		args = append(args, "-f", input.Values)
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Dir = input.Dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm install: %s: %s", err, out)
	}

	logger.Info("Helm chart deployed", "release", input.ReleaseName)
	return nil
}

// HelmTest runs Helm tests on a deployed release.
func (a *Activities) HelmTest(ctx context.Context, input HelmTestInput) (HelmTestResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Running Helm tests", "cluster", input.ClusterName, "release", input.ReleaseName)

	testCmd := input.TestCommand
	if testCmd == "" {
		testCmd = fmt.Sprintf("helm test %s --namespace %s --kubeconfig /tmp/kubeconfig-%s --timeout 5m",
			input.ReleaseName, input.Namespace, input.ClusterName)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", testCmd)
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return HelmTestResult{}, fmt.Errorf("helm test: %w", err)
		}
	}

	return HelmTestResult{
		ExitCode: exitCode,
		Output:   string(out),
	}, nil
}
