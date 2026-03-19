package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	WorkflowDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ci_workflow_duration_seconds",
		Help:    "Duration of CI workflows",
		Buckets: prometheus.ExponentialBuckets(1, 2, 12), // 1s to ~68min
	}, []string{"repo", "status"})

	StepStatus = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ci_step_status_total",
		Help: "Count of CI step results",
	}, []string{"step", "status"})

	PodsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ci_pods_active",
		Help: "Number of currently running CI pods",
	})

	WebhookRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ci_webhook_requests_total",
		Help: "Count of webhook requests",
	}, []string{"event", "status"})
)
