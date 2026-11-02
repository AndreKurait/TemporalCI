package activities

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
)

// Keep calls comfortably below RunStep's two-minute heartbeat timeout. The SDK
// may throttle delivery; cancellation remains cooperative, not instantaneous.
const stepHeartbeatInterval = 30 * time.Second

func startStepHeartbeat(ctx context.Context) func() {
	return startHeartbeat(ctx, stepHeartbeatInterval, func() {
		activity.RecordHeartbeat(ctx)
	})
}

// startHeartbeat returns a stop function that waits for the loop to exit, so no
// heartbeat calls can outlive the activity after its deferred stop returns.
func startHeartbeat(ctx context.Context, interval time.Duration, record func()) func() {
	ctx, cancel := context.WithCancel(ctx)
	if ctx.Err() == nil {
		record()
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if ctx.Err() != nil {
					return
				}
				record()
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
