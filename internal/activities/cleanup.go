package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/AndreKurait/TemporalCI/internal/k8s"
)

// CleanupPodsResult defines the output of the CleanupStalePods activity.
type CleanupPodsResult struct {
	Deleted int `json:"deleted"`
}

// CleanupStalePods removes completed CI pods older than 1 hour.
func (a *Activities) CleanupStalePods(ctx context.Context) (CleanupPodsResult, error) {
	if a.K8sClient == nil {
		return CleanupPodsResult{}, nil
	}
	deleted, err := k8s.CleanupStalePods(ctx, a.K8sClient, a.namespace(), 1*time.Hour)
	if err != nil {
		return CleanupPodsResult{}, fmt.Errorf("cleanup pods: %w", err)
	}
	a.logger(ctx).Info("cleaned up stale pods", "deleted", deleted)
	return CleanupPodsResult{Deleted: deleted}, nil
}
