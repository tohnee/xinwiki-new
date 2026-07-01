package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "xinwiki_http_requests_total",
			Help: "Total number of HTTP requests processed, labeled by method, path, and status code.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "xinwiki_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds, labeled by method and path.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	httpRequestsInFlight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "xinwiki_http_requests_in_flight",
			Help: "Current number of HTTP requests being processed, labeled by method.",
		},
		[]string{"method"},
	)

	httpRequestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "xinwiki_http_request_size_bytes",
			Help:    "HTTP request size in bytes.",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7),
		},
		[]string{"method", "path"},
	)

	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "xinwiki_http_response_size_bytes",
			Help:    "HTTP response size in bytes.",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7),
		},
		[]string{"method", "path", "status"},
	)
)

func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		reqSize := computeApproximateRequestSize(c.Request)
		httpRequestSize.WithLabelValues(method, path).Observe(float64(reqSize))
		httpRequestsInFlight.WithLabelValues(method).Inc()

		c.Next()

		httpRequestsInFlight.WithLabelValues(method).Dec()

		status := strconv.Itoa(c.Writer.Status())
		latency := time.Since(start).Seconds()
		respSize := float64(c.Writer.Size())

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(latency)
		httpResponseSize.WithLabelValues(method, path, status).Observe(respSize)
	}
}

func computeApproximateRequestSize(r *http.Request) int {
	s := 0
	if r.URL != nil {
		s += len(r.URL.String())
	}
	s += len(r.Method)
	s += len(r.Proto)
	for name, values := range r.Header {
		s += len(name)
		for _, v := range values {
			s += len(v)
		}
	}
	s += len(r.Host)
	if r.ContentLength != -1 {
		s += int(r.ContentLength)
	}
	return s
}
