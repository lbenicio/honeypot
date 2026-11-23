package metrics

import (
	"net/http"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics owns the application collectors and their HTTP endpoints.
type Metrics struct {
	registry           *prometheus.Registry
	sshConnections     prometheus.Counter
	activeConnections  prometheus.Gauge
	sshConnectionError prometheus.Counter
	passwordAttempts   prometheus.Counter
	sshReady           atomic.Bool
}

func New() *Metrics {
	registry := prometheus.NewRegistry()
	metrics := &Metrics{
		registry: registry,
		sshConnections: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fakessh_ssh_connections_total",
			Help: "Total number of accepted SSH TCP connections.",
		}),
		activeConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "fakessh_active_connections",
			Help: "Number of SSH TCP connections currently being handled.",
		}),
		sshConnectionError: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fakessh_ssh_connection_errors_total",
			Help: "Total number of SSH connections that ended with an error, including rejected authentication.",
		}),
		passwordAttempts: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fakessh_password_auth_attempts_total",
			Help: "Total number of password authentication attempts.",
		}),
	}
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		metrics.sshConnections,
		metrics.activeConnections,
		metrics.sshConnectionError,
		metrics.passwordAttempts,
	)
	return metrics
}

func (m *Metrics) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", m.healthHandler)
	return mux
}

func (m *Metrics) ConnectionAccepted() {
	m.sshConnections.Inc()
}

func (m *Metrics) ConnectionStarted() {
	m.activeConnections.Inc()
}

func (m *Metrics) ConnectionFinished() {
	m.activeConnections.Dec()
}

func (m *Metrics) ConnectionFailed() {
	m.sshConnectionError.Inc()
}

func (m *Metrics) PasswordAttempted() {
	m.passwordAttempts.Inc()
}

func (m *Metrics) SetSSHReady(ready bool) {
	m.sshReady.Store(ready)
}

func (m *Metrics) healthHandler(w http.ResponseWriter, _ *http.Request) {
	if !m.sshReady.Load() {
		http.Error(w, "SSH listener is not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
