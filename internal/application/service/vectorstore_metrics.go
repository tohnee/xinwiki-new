package service

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	readRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vectorstore_read_requests_total",
			Help: "Total number of read requests routed by the R/W splitter",
		},
		[]string{"store_id", "node_type", "consistency_level", "result"}, // node_type: master/replica; result: success/error
	)

	writeRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vectorstore_write_requests_total",
			Help: "Total number of write requests routed to master",
		},
		[]string{"store_id", "operation", "result"}, // operation: index/batch_index/delete/etc; result: success/error
	)

	requestLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "vectorstore_request_latency_seconds",
			Help:    "Latency of vectorstore requests through the R/W router",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"store_id", "operation", "node_type"},
	)

	healthyNodesGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vectorstore_healthy_nodes",
			Help: "Number of currently healthy nodes per store",
		},
		[]string{"store_id", "node_role"}, // node_role: master/replica
	)

	circuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vectorstore_circuit_breaker_state",
			Help: "Current circuit breaker state (0=closed, 1=half-open, 2=open)",
		},
		[]string{"store_id", "node_id"},
	)

	replicaLSNLag = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vectorstore_replica_lsn_lag",
			Help: "LSN lag between master and replica (higher = more behind)",
		},
		[]string{"store_id", "node_id"},
	)

	nodeHealthCheckLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "vectorstore_health_check_latency_seconds",
			Help:    "Latency of node health check requests",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"store_id", "node_id", "result"},
	)

	writeBufferSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vectorstore_write_buffer_size",
			Help: "Current number of requests waiting in the write buffer",
		},
		[]string{"store_id"},
	)
)
