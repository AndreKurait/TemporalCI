package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// PodSpec defines the configuration for a CI job pod.
type PodSpec struct {
	Name        string
	Namespace   string
	Image       string
	Command     []string
	WorkingDir  string
	Env         map[string]string
	Tolerations []string
}

// PodResult captures the outcome of a pod execution.
type PodResult struct {
	ExitCode int    `json:"exitCode"`
	Logs     string `json:"logs"`
}

// RunPod creates a K8s pod, waits for completion, collects logs, and cleans up.
func RunPod(ctx context.Context, client kubernetes.Interface, spec PodSpec) (PodResult, error) {
	pod := buildPod(spec)

	created, err := client.CoreV1().Pods(spec.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return PodResult{}, fmt.Errorf("create pod: %w", err)
	}
	defer func() {
		_ = client.CoreV1().Pods(spec.Namespace).Delete(context.Background(), created.Name, metav1.DeleteOptions{})
	}()

	if err := waitForPod(ctx, client, spec.Namespace, created.Name); err != nil {
		return PodResult{}, fmt.Errorf("wait pod: %w", err)
	}

	logs, err := getPodLogs(ctx, client, spec.Namespace, created.Name)
	if err != nil {
		return PodResult{}, fmt.Errorf("get logs: %w", err)
	}

	finished, err := client.CoreV1().Pods(spec.Namespace).Get(ctx, created.Name, metav1.GetOptions{})
	if err != nil {
		return PodResult{}, fmt.Errorf("get pod status: %w", err)
	}

	exitCode := exitCodeFromPod(finished)
	return PodResult{ExitCode: exitCode, Logs: logs}, nil
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

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers:    []corev1.Container{container},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	for _, t := range spec.Tolerations {
		if t == "ci-jobs" {
			pod.Spec.Tolerations = append(pod.Spec.Tolerations, corev1.Toleration{
				Key:      "ci-jobs",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			})
		}
	}

	return pod
}

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

func getPodLogs(ctx context.Context, client kubernetes.Interface, namespace, name string) (string, error) {
	req := client.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{})
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
