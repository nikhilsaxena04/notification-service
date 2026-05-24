package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// JobsEnqueuedTotal tracks the total number of jobs successfully enqueued.
	JobsEnqueuedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ns_jobs_enqueued_total",
			Help: "Total number of jobs enqueued",
		},
		[]string{"channel", "priority"},
	)

	// JobsProcessedTotal tracks the total number of jobs processed.
	JobsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ns_jobs_processed_total",
			Help: "Total number of jobs processed",
		},
		[]string{"channel", "status"},
	)

	// JobsDLQTotal tracks the total number of jobs escalated to DLQ.
	JobsDLQTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ns_jobs_dlq_total",
			Help: "Total number of jobs sent to dead letter queue",
		},
		[]string{"channel"},
	)

	// QueueDepth tracks the current depth of Redis queues.
	QueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ns_queue_depth",
			Help: "Current depth of the notification queues",
		},
		[]string{"queue"},
	)

	// JobDurationSeconds tracks the time taken to fully process a job.
	JobDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ns_job_duration_seconds",
			Help:    "Histogram of job processing durations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"channel"},
	)

	// DispatchDurationSeconds tracks the time taken for external provider dispatch.
	DispatchDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ns_dispatch_duration_seconds",
			Help:    "Histogram of external provider dispatch durations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"channel", "provider"},
	)
)
