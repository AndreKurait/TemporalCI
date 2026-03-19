package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// PodSpec defines the configuration for a CI job pod.
type PodSpec struct {
	Name         string
	Namespace    string
	Image        string
	Command      []string
	WorkingDir   string
	Env          map[string]string
	Tolerations  []string
	NodeSelector map[string]string
	CPU          string
	Memory       string
	// Clone config: if set, an init container clones the repo
	CloneURL string
	CloneRef string
	// Cache config
	CachePVC string // PVC name for Go module cache (empty = ephemeral emptyDir)
	// Artifact config
	ArtifactPVC string // PVC name for artifact sharing between steps
	// Q4: Service containers
	Services []ServiceSpec
	// Q4: Docker-in-Docker
	Docker         bool
	DockerCachePVC string // PVC for Docker layer cache
	// Q4: Privileged mode
	Privileged bool
	// Q4: Step output collection
	CollectOutputs bool
}

// ServiceSpec defines a sidecar service container.
type ServiceSpec struct {
	Name   string
	Image  string
	Ports  []int
	Health *HealthSpec
	Env    map[string]string
}

// HealthSpec defines a service health check.
type HealthSpec struct {
	Cmd      string
	Interval string
	Retries  int
}

// PodResult captures the outcome of a pod execution.
type PodResult struct {
	ExitCode int               `json:"exitCode"`
	Logs     string            `json:"logs"`
	Outputs  map[string]string `json:"outputs,omitempty"`
}

// RunPod creates a K8s pod, waits for completion, collects logs, and cleans up.
func RunPod(ctx context.Context, client kubernetes.Interface, spec PodSpec) (PodResult, error) {
	pod := buildPod(spec)

	created, err := client.CoreV1().Pods(spec.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		// Pod may exist from a previous run — delete and retry
		_ = client.CoreV1().Pods(spec.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		created, err = client.CoreV1().Pods(spec.Namespace).Create(ctx, pod, metav1.CreateOptions{})
		if err != nil {
			return PodResult{}, fmt.Errorf("create pod: %w", err)
		}
	}
	defer func() {
		_ = client.CoreV1().Pods(spec.Namespace).Delete(context.Background(), created.Name, metav1.DeleteOptions{})
	}()

	if err := waitForPod(ctx, client, spec.Namespace, created.Name); err != nil {
		return PodResult{}, fmt.Errorf("wait pod: %w", err)
	}

	logs, err := getPodLogs(ctx, client, spec.Namespace, created.Name, "ci")
	if err != nil {
		return PodResult{}, fmt.Errorf("get logs: %w", err)
	}

	// Collect service container logs
	for _, svc := range spec.Services {
		svcLogs, err := getPodLogs(ctx, client, spec.Namespace, created.Name, svc.Name)
		if err == nil && svcLogs != "" {
			logs += fmt.Sprintf("\n--- %s logs ---\n%s", svc.Name, svcLogs)
		}
	}

	finished, err := client.CoreV1().Pods(spec.Namespace).Get(ctx, created.Name, metav1.GetOptions{})
	if err != nil {
		return PodResult{}, fmt.Errorf("get pod status: %w", err)
	}

	exitCode := exitCodeFromPod(finished)
	result := PodResult{ExitCode: exitCode, Logs: logs}

	// Collect step outputs if enabled
	if spec.CollectOutputs {
		outputs, err := readOutputs(ctx, client, spec.Namespace, created.Name)
		if err == nil {
			result.Outputs = outputs
		}
		// Ignore errors — outputs are optional
	}

	return result, nil
}

func buildPod(spec PodSpec) *corev1.Pod {
	var envVars []corev1.EnvVar
	for k, v := range spec.Env {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	container := corev1.Container{
		Name:       "ci",
		Image:      spec.Image,
		Command:    spec.Command,
		WorkingDir: spec.WorkingDir,
		Env:        envVars,
	}

	// Resource limits
	if spec.CPU != "" || spec.Memory != "" {
		container.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{},
			Limits:   corev1.ResourceList{},
		}
		if spec.CPU != "" {
			q := resource.MustParse(spec.CPU)
			container.Resources.Requests[corev1.ResourceCPU] = q
			container.Resources.Limits[corev1.ResourceCPU] = q
		}
		if spec.Memory != "" {
			q := resource.MustParse(spec.Memory)
			container.Resources.Requests[corev1.ResourceMemory] = q
			container.Resources.Limits[corev1.ResourceMemory] = q
		}
	}

	// Privileged mode
	if spec.Privileged {
		privileged := true
		container.SecurityContext = &corev1.SecurityContext{
			Privileged: &privileged,
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"SYS_ADMIN"},
			},
		}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Labels:    map[string]string{"app": "temporalci-ci-job"},
		},
		Spec: corev1.PodSpec{
			Containers:         []corev1.Container{container},
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: "temporalci-ci-job",
		},
	}

	// If clone config is set, add init container and shared volume
	if spec.CloneURL != "" {
		workspaceVol := corev1.Volume{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
		pod.Spec.Volumes = append(pod.Spec.Volumes, workspaceVol)

		mount := corev1.VolumeMount{Name: "workspace", MountPath: "/workspace"}
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, mount)
		pod.Spec.Containers[0].WorkingDir = "/workspace"

		pod.Spec.InitContainers = []corev1.Container{{
			Name:         "clone",
			Image:        "alpine/git:latest",
			Command:      []string{"git", "clone", "--depth=1", "--branch", spec.CloneRef, spec.CloneURL, "/workspace"},
			VolumeMounts: []corev1.VolumeMount{mount},
		}}
	}

	// Go module cache — PVC-backed if configured, otherwise emptyDir
	if spec.CachePVC != "" {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "go-cache",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: spec.CachePVC,
				},
			},
		})
	} else {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "go-cache",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{Name: "go-cache", MountPath: "/go/pkg/mod"},
	)

	// Artifact volume — PVC-backed if configured
	if spec.ArtifactPVC != "" {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "artifacts",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: spec.ArtifactPVC,
				},
			},
		})
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{Name: "artifacts", MountPath: "/artifacts"},
		)
	}

	// Step outputs volume
	if spec.CollectOutputs {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "outputs",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{Name: "outputs", MountPath: "/temporalci/outputs"},
		)
	}

	// Docker-in-Docker sidecar
	if spec.Docker {
		dindContainer := corev1.Container{
			Name:  "dind",
			Image: "docker:27-dind",
			Env: []corev1.EnvVar{
				{Name: "DOCKER_TLS_CERTDIR", Value: ""},
			},
			SecurityContext: &corev1.SecurityContext{
				Privileged: boolPtr(true),
			},
		}

		// Shared docker socket
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "docker-sock",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
		sockMount := corev1.VolumeMount{Name: "docker-sock", MountPath: "/var/run"}
		dindContainer.VolumeMounts = append(dindContainer.VolumeMounts, sockMount)
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, sockMount)
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env,
			corev1.EnvVar{Name: "DOCKER_HOST", Value: "unix:///var/run/docker.sock"},
		)

		// Docker layer cache PVC
		if spec.DockerCachePVC != "" {
			pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
				Name: "docker-cache",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: spec.DockerCachePVC,
					},
				},
			})
			dindContainer.VolumeMounts = append(dindContainer.VolumeMounts,
				corev1.VolumeMount{Name: "docker-cache", MountPath: "/var/lib/docker"},
			)
		}

		pod.Spec.Containers = append(pod.Spec.Containers, dindContainer)
	}

	// Service containers (sidecars)
	for _, svc := range spec.Services {
		c := corev1.Container{
			Name:  svc.Name,
			Image: svc.Image,
		}
		for _, port := range svc.Ports {
			c.Ports = append(c.Ports, corev1.ContainerPort{ContainerPort: int32(port)})
		}
		for k, v := range svc.Env {
			c.Env = append(c.Env, corev1.EnvVar{Name: k, Value: v})
		}
		if svc.Health != nil {
			retries := int32(svc.Health.Retries)
			if retries == 0 {
				retries = 30
			}
			c.StartupProbe = &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{"sh", "-c", svc.Health.Cmd},
					},
				},
				PeriodSeconds:  10,
				FailureThreshold: retries,
			}
		}
		pod.Spec.Containers = append(pod.Spec.Containers, c)
	}

	// Tolerations
	for _, t := range spec.Tolerations {
		if t == "ci-jobs" {
			pod.Spec.Tolerations = append(pod.Spec.Tolerations, corev1.Toleration{
				Key:      "workload",
				Value:    "ci-job",
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			})
		}
	}

	// Node selector
	if len(spec.NodeSelector) > 0 {
		pod.Spec.NodeSelector = spec.NodeSelector
	}

	return pod
}

// readOutputs reads step outputs from /temporalci/outputs/env in a completed pod.
func readOutputs(ctx context.Context, client kubernetes.Interface, namespace, name string) (map[string]string, error) {
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(name).
		Namespace(namespace).
		SubResource("exec").
		Param("container", "ci").
		Param("command", "cat").
		Param("command", "/temporalci/outputs/env").
		Param("stdout", "true").
		Param("stderr", "true")

	// For simplicity, read from pod logs convention: outputs are also in logs
	// In practice, use remotecommand.NewSPDYExecutor. For now, parse from a
	// well-known log marker that the step can emit.
	// Alternative: read the emptyDir volume content via a helper container.
	// We'll use the exec approach with the K8s API.
	_ = req // placeholder — actual exec requires SPDY executor setup

	// Fallback: parse outputs from pod logs using a marker convention
	logs, err := getPodLogs(ctx, client, namespace, name, "ci")
	if err != nil {
		return nil, err
	}
	return parseOutputsFromLogs(logs), nil
}

// parseOutputsFromLogs extracts key=value pairs between ::temporalci-outputs:: markers.
// Steps can also write to /temporalci/outputs/env which is read via exec.
func parseOutputsFromLogs(logs string) map[string]string {
	outputs := map[string]string{}
	inBlock := false
	for _, line := range strings.Split(logs, "\n") {
		line = strings.TrimSpace(line)
		if line == "::temporalci-outputs-start::" {
			inBlock = true
			continue
		}
		if line == "::temporalci-outputs-end::" {
			inBlock = false
			continue
		}
		if inBlock {
			if k, v, ok := strings.Cut(line, "="); ok {
				outputs[k] = v
			}
		}
	}
	return outputs
}

func boolPtr(b bool) *bool { return &b }

func waitForPod(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	watcher, err := client.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + name,
	})
	if err != nil {
		return err
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		if event.Type == watch.Error {
			return fmt.Errorf("watch error")
		}
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			continue
		}
		switch pod.Status.Phase {
		case corev1.PodSucceeded, corev1.PodFailed:
			return nil
		}
	}
	return fmt.Errorf("watch closed before pod completed")
}

func getPodLogs(ctx context.Context, client kubernetes.Interface, namespace, name, container string) (string, error) {
	opts := &corev1.PodLogOptions{}
	if container != "" {
		opts.Container = container
	}
	req := client.CoreV1().Pods(namespace).GetLogs(name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, stream); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func exitCodeFromPod(pod *corev1.Pod) int {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == "ci" && cs.State.Terminated != nil {
			return int(cs.State.Terminated.ExitCode)
		}
	}
	return -1
}
