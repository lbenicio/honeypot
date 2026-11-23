package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerExportsMetricsAndReadiness(t *testing.T) {
	appMetrics := New()
	handler := appMetrics.Handler()

	healthRecorder := httptest.NewRecorder()
	handler.ServeHTTP(healthRecorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if healthRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("initial health status = %d, want %d", healthRecorder.Code, http.StatusServiceUnavailable)
	}

	appMetrics.ConnectionAccepted()
	appMetrics.ConnectionStarted()
	appMetrics.PasswordAttempted()
	appMetrics.ConnectionFailed()
	appMetrics.ConnectionFinished()

	metricsRecorder := httptest.NewRecorder()
	handler.ServeHTTP(metricsRecorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if metricsRecorder.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d", metricsRecorder.Code, http.StatusOK)
	}
	for _, expected := range []string{
		"fakessh_ssh_connections_total 1",
		"fakessh_active_connections 0",
		"fakessh_ssh_connection_errors_total 1",
		"fakessh_password_auth_attempts_total 1",
	} {
		if !strings.Contains(metricsRecorder.Body.String(), expected) {
			t.Errorf("metrics response does not contain %q", expected)
		}
	}

	appMetrics.SetSSHReady(true)
	healthRecorder = httptest.NewRecorder()
	handler.ServeHTTP(healthRecorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if healthRecorder.Code != http.StatusOK {
		t.Errorf("ready health status = %d, want %d", healthRecorder.Code, http.StatusOK)
	}
}
