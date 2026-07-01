package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestMetricsMiddleware_RecordRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(PrometheusMetrics())
	r.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.POST("/api/create", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"created": true})
	})
	r.GET("/api/error", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail"})
	})

	// Make requests
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w1.Code)
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/create", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w2.Code)
	}

	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/api/error", nil)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w3.Code)
	}

	// Verify request counter recorded
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, mf := range metrics {
		if mf.GetName() == "xinwiki_http_requests_total" {
			found = true
			// Should have at least 3 time series (GET/200, POST/201, GET/500)
			if len(mf.GetMetric()) < 3 {
				t.Errorf("expected at least 3 request series, got %d", len(mf.GetMetric()))
			}
			// Sum should be 3
			total := float64(0)
			for _, m := range mf.GetMetric() {
				total += m.GetCounter().GetValue()
				// Verify labels exist
				labels := labelMap(m)
				if _, ok := labels["method"]; !ok {
					t.Error("missing method label")
				}
				if _, ok := labels["path"]; !ok {
					t.Error("missing path label")
				}
				if _, ok := labels["status"]; !ok {
					t.Error("missing status label")
				}
			}
			if total < 3 {
				t.Errorf("expected total requests >= 3, got %f", total)
			}
		}
	}
	if !found {
		t.Fatal("xinwiki_http_requests_total metric not found")
	}
}

func TestMetricsMiddleware_LatencyHistogram(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(PrometheusMetrics())
	r.GET("/api/fast", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/fast", nil)
	r.ServeHTTP(w, req)

	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, mf := range metrics {
		if mf.GetName() == "xinwiki_http_request_duration_seconds" {
			found = true
			for _, m := range mf.GetMetric() {
				h := m.GetHistogram()
				if h == nil {
					t.Fatal("expected histogram type")
				}
				if h.GetSampleCount() < 1 {
					t.Error("expected at least 1 sample in histogram")
				}
			}
		}
	}
	if !found {
		t.Fatal("xinwiki_http_request_duration_seconds metric not found")
	}
}

func TestMetricsMiddleware_InFlight(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Make a request and verify in-flight metric exists
	r := gin.New()
	r.Use(PrometheusMetrics())
	r.GET("/api/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	r.ServeHTTP(w, req)

	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, mf := range metrics {
		if mf.GetName() == "xinwiki_http_requests_in_flight" {
			found = true
			for _, m := range mf.GetMetric() {
				if m.GetGauge() == nil {
					t.Fatal("expected gauge type for in-flight")
				}
			}
		}
	}
	if !found {
		t.Fatal("xinwiki_http_requests_in_flight metric not found")
	}
}

func TestMetricsMiddleware_ResponseSize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(PrometheusMetrics())
	r.GET("/api/data", func(c *gin.Context) {
		c.String(http.StatusOK, strings.Repeat("x", 1024))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	r.ServeHTTP(w, req)

	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, mf := range metrics {
		if mf.GetName() == "xinwiki_http_response_size_bytes" {
			found = true
			for _, m := range mf.GetMetric() {
				h := m.GetHistogram()
				if h == nil {
					t.Fatal("expected histogram for response size")
				}
				if h.GetSampleCount() < 1 {
					t.Error("expected at least 1 response size sample")
				}
			}
		}
	}
	if !found {
		t.Fatal("xinwiki_http_response_size_bytes metric not found")
	}
}

func TestMetricsMiddleware_RequestSize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(PrometheusMetrics())
	r.POST("/api/upload", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"received": true})
	})

	body := strings.Repeat("y", 512)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/upload", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	r.ServeHTTP(w, req)

	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, mf := range metrics {
		if mf.GetName() == "xinwiki_http_request_size_bytes" {
			found = true
			for _, m := range mf.GetMetric() {
				h := m.GetHistogram()
				if h == nil {
					t.Fatal("expected histogram for request size")
				}
				if h.GetSampleCount() < 1 {
					t.Error("expected at least 1 request size sample")
				}
			}
		}
	}
	if !found {
		t.Fatal("xinwiki_http_request_size_bytes metric not found")
	}
}

func labelMap(m *dto.Metric) map[string]string {
	lm := make(map[string]string, len(m.GetLabel()))
	for _, lp := range m.GetLabel() {
		lm[lp.GetName()] = lp.GetValue()
	}
	return lm
}
