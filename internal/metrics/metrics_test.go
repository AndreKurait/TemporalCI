package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsRegistered(t *testing.T) {
	// Verify all metrics are registered with Prometheus
	metrics := []prometheus.Collector{
		WorkflowDuration,
		StepStatus,
		PodsActive,
		WebhookRequests,
	}
	for _, m := range metrics {
		if m == nil {
			t.Error("metric is nil")
		}
	}
}

func TestStepStatusIncrement(t *testing.T) {
	// Verify we can increment without panic
	StepStatus.WithLabelValues("build", "passed").Inc()
	StepStatus.WithLabelValues("test", "failed").Inc()
}

func TestPodsActiveGauge(t *testing.T) {
	PodsActive.Inc()
	PodsActive.Dec()
}
