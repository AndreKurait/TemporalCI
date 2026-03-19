package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

// LockRequest is sent via signal to acquire a lock.
type LockRequest struct {
	Resource   string `json:"resource"`
	Requester  string `json:"requester"` // workflow ID
	Pool       string `json:"pool,omitempty"`
	Quantity   int    `json:"quantity,omitempty"`
}

// LockResponse is returned to the requester.
type LockResponse struct {
	Acquired bool   `json:"acquired"`
	Resource string `json:"resource"` // actual resource name (for pool locks)
}

// LockState tracks a single held lock.
type LockState struct {
	Resource  string    `json:"resource"`
	Holder    string    `json:"holder"`
	AcquiredAt time.Time `json:"acquiredAt"`
}

// LockManagerState is queryable state.
type LockManagerState struct {
	Locks []LockState `json:"locks"`
	Queue []LockRequest `json:"queue"`
}

// LockManager is a long-running workflow that manages exclusive resource locks.
// Pipelines signal to acquire/release. State survives restarts via Temporal durability.
func LockManager(ctx workflow.Context) error {
	locks := map[string]string{}    // resource → holder workflow ID
	pools := map[string][]string{}  // pool label → resource names
	var queue []LockRequest
	var state LockManagerState

	// Query handler for dashboard
	_ = workflow.SetQueryHandler(ctx, "state", func() (LockManagerState, error) {
		state.Locks = nil
		for res, holder := range locks {
			state.Locks = append(state.Locks, LockState{Resource: res, Holder: holder})
		}
		state.Queue = queue
		return state, nil
	})

	acquireCh := workflow.GetSignalChannel(ctx, "acquire")
	releaseCh := workflow.GetSignalChannel(ctx, "release")
	registerPoolCh := workflow.GetSignalChannel(ctx, "register-pool")

	for {
		sel := workflow.NewSelector(ctx)

		sel.AddReceive(acquireCh, func(ch workflow.ReceiveChannel, more bool) {
			var req LockRequest
			ch.Receive(ctx, &req)

			if req.Pool != "" {
				// Pool lock: find any available resource
				if resources, ok := pools[req.Pool]; ok {
					for _, res := range resources {
						if _, held := locks[res]; !held {
							locks[res] = req.Requester
							// Signal back to requester
							workflow.SignalExternalWorkflow(ctx, req.Requester, "", "lock-acquired", LockResponse{Acquired: true, Resource: res})
							return
						}
					}
				}
				queue = append(queue, req)
				return
			}

			// Named lock
			if _, held := locks[req.Resource]; !held {
				locks[req.Resource] = req.Requester
				workflow.SignalExternalWorkflow(ctx, req.Requester, "", "lock-acquired", LockResponse{Acquired: true, Resource: req.Resource})
			} else {
				queue = append(queue, req)
			}
		})

		sel.AddReceive(releaseCh, func(ch workflow.ReceiveChannel, more bool) {
			var req LockRequest
			ch.Receive(ctx, &req)

			resource := req.Resource
			if resource == "" {
				// Find resource held by this requester
				for res, holder := range locks {
					if holder == req.Requester {
						resource = res
						break
					}
				}
			}
			delete(locks, resource)

			// Process queue — grant to next waiter
			var remaining []LockRequest
			granted := false
			for _, q := range queue {
				if granted {
					remaining = append(remaining, q)
					continue
				}
				if q.Pool != "" {
					if resources, ok := pools[q.Pool]; ok {
						for _, res := range resources {
							if _, held := locks[res]; !held {
								locks[res] = q.Requester
								workflow.SignalExternalWorkflow(ctx, q.Requester, "", "lock-acquired", LockResponse{Acquired: true, Resource: res})
								granted = true
								break
							}
						}
					}
					if !granted {
						remaining = append(remaining, q)
					}
				} else if q.Resource == resource {
					locks[resource] = q.Requester
					workflow.SignalExternalWorkflow(ctx, q.Requester, "", "lock-acquired", LockResponse{Acquired: true, Resource: resource})
					granted = true
				} else {
					remaining = append(remaining, q)
				}
			}
			queue = remaining
		})

		sel.AddReceive(registerPoolCh, func(ch workflow.ReceiveChannel, more bool) {
			var pool struct {
				Label     string   `json:"label"`
				Resources []string `json:"resources"`
			}
			ch.Receive(ctx, &pool)
			pools[pool.Label] = pool.Resources
		})

		sel.Select(ctx)

		// ContinueAsNew every 10000 signals to prevent history blow-up
		info := workflow.GetInfo(ctx)
		if info.GetCurrentHistoryLength() > 10000 {
			return workflow.NewContinueAsNewError(ctx, LockManager)
		}
	}
}

// AcquireLock signals the lock manager and waits for acquisition.
func AcquireLock(ctx workflow.Context, resource, pool string, timeout time.Duration) (string, error) {
	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID

	req := LockRequest{Resource: resource, Pool: pool, Requester: wfID}
	_ = workflow.SignalExternalWorkflow(ctx, "lock-manager", "", "acquire", req).Get(ctx, nil)

	// Wait for lock-acquired signal
	ch := workflow.GetSignalChannel(ctx, "lock-acquired")
	var resp LockResponse

	timerCtx, cancel := workflow.WithCancel(ctx)
	timer := workflow.NewTimer(timerCtx, timeout)

	sel := workflow.NewSelector(ctx)
	sel.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &resp)
		cancel()
	})
	sel.AddFuture(timer, func(f workflow.Future) {
		resp.Acquired = false
	})
	sel.Select(ctx)

	if !resp.Acquired {
		return "", fmt.Errorf("lock acquisition timed out after %v", timeout)
	}
	return resp.Resource, nil
}

// ReleaseLock signals the lock manager to release.
func ReleaseLock(ctx workflow.Context, resource string) {
	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	_ = workflow.SignalExternalWorkflow(ctx, "lock-manager", "", "release", LockRequest{Resource: resource, Requester: wfID}).Get(ctx, nil)
}
