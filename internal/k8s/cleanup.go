package k8s

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CleanupStalePods deletes completed CI pods older than maxAge.
func CleanupStalePods(ctx context.Context, client kubernetes.Interface, namespace string, maxAge time.Duration) (int, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=temporalci-ci-job",
	})
	if err != nil {
		return 0, err
	}

	deleted := 0
	cutoff := time.Now().Add(-maxAge)
	for _, pod := range pods.Items {
		if pod.Status.Phase != "Succeeded" && pod.Status.Phase != "Failed" {
			continue
		}
		if pod.CreationTimestamp.Time.Before(cutoff) {
			if err := client.CoreV1().Pods(namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err == nil {
				deleted++
			}
		}
	}
	return deleted, nil
}
