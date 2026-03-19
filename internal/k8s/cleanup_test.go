package k8s

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCleanupStalePods(t *testing.T) {
	client := fake.NewSimpleClientset()
	ns := "default"

	// Create an old completed pod
	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "ci-old",
			Namespace:         ns,
			Labels:            map[string]string{"app": "temporalci-ci-job"},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	// Create a recent completed pod
	recentPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "ci-recent",
			Namespace:         ns,
			Labels:            map[string]string{"app": "temporalci-ci-job"},
			CreationTimestamp: metav1.NewTime(time.Now()),
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	// Create a running pod
	runningPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "ci-running",
			Namespace:         ns,
			Labels:            map[string]string{"app": "temporalci-ci-job"},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	ctx := context.Background()
	for _, p := range []*corev1.Pod{oldPod, recentPod, runningPod} {
		if _, err := client.CoreV1().Pods(ns).Create(ctx, p, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := CleanupStalePods(ctx, client, ns, 1*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	// Verify remaining pods
	pods, _ := client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if len(pods.Items) != 2 {
		t.Errorf("remaining pods = %d, want 2", len(pods.Items))
	}
}

func TestCleanupStalePods_Empty(t *testing.T) {
	client := fake.NewSimpleClientset()
	deleted, err := CleanupStalePods(context.Background(), client, "default", 1*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
}
