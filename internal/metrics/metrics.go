package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Namespace for all metrics
const namespace = "adk_agent"

var (
	// DistillDuration tracks the latency of the distillation process steps.
	DistillDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "distill",
		Name:      "duration_seconds",
		Help:      "Duration of distillation steps in seconds",
		Buckets:   prometheus.DefBuckets,
	}, []string{"step", "status"})

	// DistillItemsCount tracks the number of items processed/generated.
	DistillItemsCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "distill",
		Name:      "items_total",
		Help:      "Total number of items processed or extracted",
	}, []string{"type"})
)
