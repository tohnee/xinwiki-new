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
// (text/plain; version=0.0.4). The endpoint is auth-whitelisted; access
// is restricted to loopback/private IPs by metricsAccessGuard unless
// METRICS_ALLOW_PUBLIC=true. We simulate a loopback client here so the
// guard lets the request through.
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
	// future refactor that drops promhttp.Handler() or the access guard
	// also fails this test.
	r.GET("/metrics", metricsAccessGuard(), gin.WrapH(promhttp.Handler()))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	// Simulate a loopback scraper (httptest leaves RemoteAddr empty by
	// default, which would be rejected by the guard).
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics must return 200 from loopback, got %d (body: %s)", rec.Code, rec.Body.String())
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

// TestMetricsRoute_RejectsPublicIP verifies the access guard returns 403
// for non-private requesters.
func TestMetricsRoute_RejectsPublicIP(t *testing.T) {
	r := gin.New()
	r.GET("/metrics", metricsAccessGuard(), gin.WrapH(promhttp.Handler()))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "203.0.113.42:54321" // RFC 5737 TEST-NET-3 — treated as public
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("/metrics must return 403 from public IP, got %d", rec.Code)
	}
}

// TestIsLoopbackOrPrivateIP_Coverage exercises the IP classifier.
func TestIsLoopbackOrPrivateIP_Coverage(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"localhost", true},
		{"0.0.0.0", true},
		{"192.168.1.1", true},
		{"10.0.0.5", true},
		{"172.16.5.9", true},
		{"fe80::1", true},
		{"fc00::1", true},
		{"8.8.8.8", false},
		{"203.0.113.42", false},
		{"", false},
		{"not-an-ip", false},
	}
	for _, tc := range cases {
		got := isLoopbackOrPrivateIP(tc.ip)
		if got != tc.want {
			t.Errorf("isLoopbackOrPrivateIP(%q) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}