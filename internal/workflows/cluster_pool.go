package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/AndreKurait/TemporalCI/internal/activities"
)

// ClusterPoolInput configures the cluster pool.
type ClusterPoolInput struct {
	DesiredSize    int      `json:"desiredSize"`    // default 1
	MaxSize        int      `json:"maxSize"`        // default 3
	ClusterTTL     string   `json:"clusterTTL"`     // default "4h"
	Region         string   `json:"region"`
	SubnetIDs      []string `json:"subnetIDs"`
	ClusterRoleARN string   `json:"clusterRoleARN"`
	NodeRoleARN    string   `json:"nodeRoleARN"`
}

// LeaseSignal requests a cluster from the pool.
type LeaseSignal struct {
	RequestID  string `json:"requestID"`
	WorkflowID string `json:"workflowID"` // caller's workflow ID to signal back
}

// ReleaseSignal returns a cluster to the pool.
type ReleaseSignal struct {
	ClusterName string `json:"clusterName"`
}

// LeaseResult is signaled back to the requesting workflow.
type LeaseResult struct {
	ClusterName string `json:"clusterName"`
	Kubeconfig  string `json:"kubeconfig"`
	Endpoint    string `json:"endpoint"`
}

// PoolStatus is returned by the "pool-status" query.
type PoolStatus struct {
	Available int `json:"available"`
	Leased    int `json:"leased"`
	Total     int `json:"total"`
}

type poolCluster struct {
	result    activities.ClusterResult
	createdAt time.Time
	leased    bool
}

// ClusterPool is a long-running workflow that manages a warm pool of EKS clusters.
func ClusterPool(ctx workflow.Context, input ClusterPoolInput) error {
	if input.DesiredSize <= 0 {
		input.DesiredSize = 1
	}
	if input.MaxSize <= 0 {
		input.MaxSize = 3
	}
	ttl := ParseTimeout(input.ClusterTTL, 4*time.Hour)

	var acts *activities.Activities
	clusters := map[string]*poolCluster{}

	clusterOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 20 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 2},
	}

	nextID := 0
	newClusterName := func() string {
		nextID++
		return fmt.Sprintf("ci-pool-%s-%d", workflow.GetInfo(ctx).WorkflowExecution.ID[:8], nextID)
	}

	// Query: pool status
	_ = workflow.SetQueryHandler(ctx, "pool-status", func() (PoolStatus, error) {
		avail, leased := 0, 0
		for _, c := range clusters {
			if c.leased {
				leased++
			} else {
				avail++
			}
		}
		return PoolStatus{Available: avail, Leased: leased, Total: len(clusters)}, nil
	})

	// Provision initial pool
	for i := 0; i < input.DesiredSize && i < input.MaxSize; i++ {
		name := newClusterName()
		provisionAsync(ctx, clusterOpts, acts, input, name, clusters)
	}

	leaseCh := workflow.GetSignalChannel(ctx, "lease")
	releaseCh := workflow.GetSignalChannel(ctx, "release")

	for {
		selector := workflow.NewSelector(ctx)

		// Handle lease requests
		selector.AddReceive(leaseCh, func(ch workflow.ReceiveChannel, _ bool) {
			var sig LeaseSignal
			ch.Receive(ctx, &sig)

			// Find an available cluster
			for name, c := range clusters {
				if !c.leased {
					c.leased = true
					// Signal the requesting workflow with the result
					workflow.SignalExternalWorkflow(ctx, sig.WorkflowID, "", "lease-result",
						LeaseResult{ClusterName: name, Kubeconfig: c.result.Kubeconfig, Endpoint: c.result.Endpoint})

					// Replenish if below desired size
					avail := 0
					for _, cc := range clusters {
						if !cc.leased {
							avail++
						}
					}
					if avail < input.DesiredSize && len(clusters) < input.MaxSize {
						n := newClusterName()
						provisionAsync(ctx, clusterOpts, acts, input, n, clusters)
					}
					return
				}
			}
			// No cluster available — provision one on demand
			if len(clusters) < input.MaxSize {
				n := newClusterName()
				provisionAsync(ctx, clusterOpts, acts, input, n, clusters)
				// The requester will need to wait; we'll signal when ready
				// For now, queue the request by re-signaling after a delay
				workflow.Go(ctx, func(gCtx workflow.Context) {
					_ = workflow.Sleep(gCtx, 60*time.Second)
					workflow.SignalExternalWorkflow(gCtx, workflow.GetInfo(gCtx).WorkflowExecution.ID, "",
						"lease", sig) // re-enqueue
				})
			}
		})

		// Handle release
		selector.AddReceive(releaseCh, func(ch workflow.ReceiveChannel, _ bool) {
			var sig ReleaseSignal
			ch.Receive(ctx, &sig)
			if c, ok := clusters[sig.ClusterName]; ok {
				c.leased = false
			}
		})

		// TTL check every 10 minutes
		selector.AddFuture(workflow.NewTimer(ctx, 10*time.Minute), func(f workflow.Future) {
			_ = f.Get(ctx, nil)
			now := workflow.Now(ctx)
			for name, c := range clusters {
				if !c.leased && now.Sub(c.createdAt) > ttl {
					// Destroy expired cluster
					destroyCtx := workflow.WithActivityOptions(ctx, clusterOpts)
					workflow.ExecuteActivity(destroyCtx, acts.DestroyCluster,
						activities.DestroyClusterInput{Name: name, Region: input.Region})
					delete(clusters, name)
				}
			}
		})

		selector.Select(ctx)
	}
}

func provisionAsync(ctx workflow.Context, opts workflow.ActivityOptions, acts *activities.Activities,
	input ClusterPoolInput, name string, clusters map[string]*poolCluster) {
	workflow.Go(ctx, func(gCtx workflow.Context) {
		provCtx := workflow.WithActivityOptions(gCtx, opts)
		var result activities.ClusterResult
		err := workflow.ExecuteActivity(provCtx, acts.ProvisionCluster, activities.ClusterInput{
			Name: name, Region: input.Region, SubnetIDs: input.SubnetIDs,
			RoleARN: input.ClusterRoleARN, NodeRoleARN: input.NodeRoleARN,
		}).Get(gCtx, &result)
		if err == nil {
			clusters[name] = &poolCluster{result: result, createdAt: workflow.Now(gCtx)}
		}
	})
}
