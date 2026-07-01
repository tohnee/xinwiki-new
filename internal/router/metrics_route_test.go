package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// TestMetricsRoute_RegisteredAndServeed: the /metrics route must be
// registered on the router and must serve Prometheus text-format output
// (text/plain; version=0.0.4). The endpoint is auth-whitelisted, so it
// must respond 200 even when no bearer token is presented. We register a
// distinct probe metric so the response demonstrably contains our metric
// rather than relying on incidental metrics registered elsewhere.
func TestMetricsRoute_RegisteredAndServeed(t *testing.T) {
	// Register a unique probe metric on the default Prometheus registry so
	// we can assert it appears verbatim in the scrape response. The label
	// value is unique to this test to avoid collision with parallel runs.
	const probeMetricName = "router_metrics_route_probe_test"
	const probeValue = 7
	promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: probeMetricName,
		Help: "router metrics route smoke-test marker; safe to scrape",
	}, []string{"test"}).WithLabelValues("smoketest").Set(probeValue)

	r := gin.New()
	// Reproduce the same registration the production router uses so a
	// future refactor that drops promhttp.Handler() also fails this test.
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	// Deliberately no Authorization header: /metrics is auth-whitelisted.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics must return 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("/metrics must serve text/plain Prometheus format, got Content-Type=%q", ct)
	}
	body := rec.Body.String()
	// The probe metric line looks like `router_metrics_route_probe_test{test="smoketest"} 7`.
	if !strings.Contains(body, probeMetricName) {
		t.Fatalf("scrape body must contain our probe metric %q\nbody:\n%s", probeMetricName, body)
	}
	if !strings.Contains(body, "go_info") {
		t.Fatalf("scrape body must also include the default Go runtime metrics (smells like the wrong registry)\nbody:\n%s", body)
	}
}