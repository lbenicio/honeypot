package main

import (
	"bytes"
	"crypto"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"golang.org/x/crypto/ssh"
)

func TestMainUsesArgumentsEnvironmentAndReportsErrors(t *testing.T) {
	oldArgs := os.Args
	oldRun := runApplication
	oldOutput := log.Writer()
	oldFlags := log.Flags()
	t.Cleanup(func() {
		os.Args = oldArgs
		runApplication = oldRun
		log.SetOutput(oldOutput)
		log.SetFlags(oldFlags)
	})

	os.Args = []string{"fakessh", "/tmp/logs"}
	t.Setenv("PROMETHEUS_ADDR", "127.0.0.1:9211")
	var output bytes.Buffer
	log.SetOutput(&output)
	called := false
	runApplication = func(args []string, metricsAddr string) error {
		called = true
		if len(args) != 1 || args[0] != "/tmp/logs" {
			t.Errorf("runApplication args = %v, want [/tmp/logs]", args)
		}
		if metricsAddr != "127.0.0.1:9211" {
			t.Errorf("metrics address = %q, want 127.0.0.1:9211", metricsAddr)
		}
		return errors.New("startup failed")
	}

	main()
	if !called {
		t.Fatal("main did not call runApplication")
	}
	if !strings.Contains(output.String(), "startup failed") {
		t.Fatalf("main did not report startup error: %q", output.String())
	}

	runApplication = func([]string, string) error { return nil }
	main()
}

func TestRunApplicationUsesDefaultDependencies(t *testing.T) {
	oldDependencies := defaultRuntimeDependencies
	t.Cleanup(func() { defaultRuntimeDependencies = oldDependencies })
	wantErr := errors.New("configuration unavailable")
	defaultRuntimeDependencies = testRuntimeDependencies()
	defaultRuntimeDependencies.newServerConfig = func() (*ssh.ServerConfig, error) {
		return nil, wantErr
	}

	err := runApplication(nil, ":0")
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestRunApplicationWithDependencies(t *testing.T) {
	oldOutput := log.Writer()
	oldFlags := log.Flags()
	t.Cleanup(func() {
		log.SetOutput(oldOutput)
		log.SetFlags(oldFlags)
	})
	log.SetOutput(io.Discard)

	t.Run("defaults metrics address, configures logging, and accepts connections", func(t *testing.T) {
		previousOutput := log.Writer()
		t.Cleanup(func() { log.SetOutput(previousOutput) })
		logDir := t.TempDir()
		fixedTime := time.Date(2026, time.November, 22, 12, 34, 56, 789000000, time.UTC)
		serverConn, clientConn := net.Pipe()
		notifiedConn := &closeNotifyConn{Conn: serverConn, closed: make(chan struct{})}
		listener := &stubListener{conn: notifiedConn, acceptErr: errors.New("listener stopped")}
		metricsResult := make(chan struct {
			addr string
			body string
		}, 1)
		deps := runtimeDependencies{
			now: func() time.Time { return fixedTime },
			openFile: func(name string, flag int, perm os.FileMode) (*os.File, error) {
				want := filepath.Join(logDir, "fakessh-2026-11-22-12-34-56-000.log")
				if name != want {
					t.Errorf("log file path = %q, want %q", name, want)
				}
				return os.OpenFile(name, flag, perm)
			},
			listen: func(network, address string) (net.Listener, error) {
				if network != "tcp" || address != ":22" {
					t.Errorf("listen(%q, %q), want tcp :22", network, address)
				}
				return listener, nil
			},
			newServerConfig: func() (*ssh.ServerConfig, error) {
				return &ssh.ServerConfig{}, nil
			},
			serveMetrics: func(addr string, handler http.Handler) error {
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
				metricsResult <- struct {
					addr string
					body string
				}{addr: addr, body: recorder.Body.String()}
				return nil
			},
		}

		if err := runApplicationWithDependencies([]string{logDir}, "", deps); err != nil {
			t.Fatalf("runApplicationWithDependencies() error = %v", err)
		}
		clientConn.Close()
		select {
		case <-notifiedConn.closed:
		case <-time.After(time.Second):
			t.Fatal("SSH connection handler did not close the connection")
		}
		select {
		case result := <-metricsResult:
			if result.addr != ":2112" {
				t.Errorf("metrics address = %q, want :2112", result.addr)
			}
			if !strings.Contains(result.body, "fakessh_ssh_connections_total") {
				t.Errorf("metrics response is missing SSH metrics: %q", result.body)
			}
		case <-time.After(time.Second):
			t.Fatal("metrics server was not started")
		}
		if _, err := os.Stat(filepath.Join(logDir, "fakessh-2026-11-22-12-34-56-000.log")); err != nil {
			t.Errorf("log file was not created: %v", err)
		}
		if got := metricValue(t, sshConnections); got < 1 {
			t.Errorf("SSH connection counter = %v, want at least 1", got)
		}
	})

	t.Run("returns configuration error", func(t *testing.T) {
		wantErr := errors.New("bad SSH config")
		deps := testRuntimeDependencies()
		deps.newServerConfig = func() (*ssh.ServerConfig, error) { return nil, wantErr }
		err := runApplicationWithDependencies(nil, "127.0.0.1:9211", deps)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want wrapped %v", err, wantErr)
		}
	})

	t.Run("returns listener error", func(t *testing.T) {
		wantErr := errors.New("port unavailable")
		deps := testRuntimeDependencies()
		deps.listen = func(string, string) (net.Listener, error) { return nil, wantErr }
		err := runApplicationWithDependencies(nil, "127.0.0.1:9212", deps)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want wrapped %v", err, wantErr)
		}
	})

	t.Run("returns log file error", func(t *testing.T) {
		wantErr := errors.New("disk unavailable")
		deps := testRuntimeDependencies()
		deps.openFile = func(string, int, os.FileMode) (*os.File, error) { return nil, wantErr }
		err := runApplicationWithDependencies([]string{"/logs"}, ":2112", deps)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want wrapped %v", err, wantErr)
		}
	})
}

func TestStartMetricsServerLogsServeError(t *testing.T) {
	oldOutput := log.Writer()
	t.Cleanup(func() { log.SetOutput(oldOutput) })
	var output bytes.Buffer
	log.SetOutput(&output)
	finished := make(chan struct{})
	startMetricsServer("127.0.0.1:0", func(addr string, handler http.Handler) error {
		if addr != "127.0.0.1:0" {
			t.Errorf("address = %q, want 127.0.0.1:0", addr)
		}
		if handler == nil {
			t.Error("metrics handler is nil")
		}
		return errors.New("serve failed")
	}, func(format string, args ...any) {
		log.Printf(format, args...)
		close(finished)
	})
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("metrics server callback was not invoked")
	}
	if !strings.Contains(output.String(), "serve failed") {
		t.Fatalf("metrics server error was not logged: %q", output.String())
	}
}

func TestNewSSHServerConfig(t *testing.T) {
	config, err := newSSHServerConfig()
	if err != nil {
		t.Fatalf("newSSHServerConfig() error = %v", err)
	}
	if config.MaxAuthTries != 6 {
		t.Errorf("MaxAuthTries = %d, want 6", config.MaxAuthTries)
	}
	if config.ServerVersion != serverVersions[0] {
		t.Errorf("ServerVersion = %q, want %q", config.ServerVersion, serverVersions[0])
	}
	if config.PasswordCallback == nil {
		t.Error("PasswordCallback is nil")
	}
}

func TestNewSSHServerConfigReturnsKeyGenerationError(t *testing.T) {
	wantErr := errors.New("random source failed")
	_, err := newSSHHostSignerWithFunctions(
		func() (crypto.Signer, error) { return nil, wantErr },
		func(crypto.Signer) (ssh.Signer, error) { return nil, errors.New("must not be called") },
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestNewSSHHostSignerReturnsSignerError(t *testing.T) {
	wantErr := errors.New("signer creation failed")
	_, err := newSSHHostSignerWithFunctions(
		func() (crypto.Signer, error) { return nil, nil },
		func(crypto.Signer) (ssh.Signer, error) { return nil, wantErr },
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestNewSSHServerConfigReturnsSignerError(t *testing.T) {
	wantErr := errors.New("host key unavailable")
	_, err := newSSHServerConfigWithSigner(func() (ssh.Signer, error) { return nil, wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestPasswordCallbackRejectsAndCountsAttempts(t *testing.T) {
	oldDelay := passwordCallbackDelay
	oldOutput := log.Writer()
	oldFlags := log.Flags()
	passwordCallbackDelay = 0
	log.SetOutput(io.Discard)
	t.Cleanup(func() {
		passwordCallbackDelay = oldDelay
		log.SetOutput(oldOutput)
		log.SetFlags(oldFlags)
	})

	before := metricValue(t, passwordAttempts)
	permissions, err := passwordCallback(stubConnMetadata{}, []byte("incorrect"))
	if permissions != nil {
		t.Errorf("permissions = %v, want nil", permissions)
	}
	if err != errBadPassword {
		t.Errorf("error = %v, want %v", err, errBadPassword)
	}
	if got := metricValue(t, passwordAttempts); got != before+1 {
		t.Errorf("password attempts = %v, want %v", got, before+1)
	}
}

func TestHandleConnCountsErrorsAndClosesConnection(t *testing.T) {
	oldOutput := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(oldOutput) })

	beforeErrors := metricValue(t, sshConnectionErrors)
	beforeActive := metricValue(t, activeConnections)
	serverConn, clientConn := net.Pipe()
	notifiedConn := &closeNotifyConn{Conn: serverConn, closed: make(chan struct{})}
	activeConnections.Inc()
	go handleConn(notifiedConn, &ssh.ServerConfig{})
	clientConn.Close()
	select {
	case <-notifiedConn.closed:
	case <-time.After(time.Second):
		t.Fatal("handleConn did not close the connection")
	}
	if got := metricValue(t, sshConnectionErrors); got != beforeErrors+1 {
		t.Errorf("connection errors = %v, want %v", got, beforeErrors+1)
	}
	if got := metricValue(t, activeConnections); got != beforeActive {
		t.Errorf("active connections = %v, want restored value %v", got, beforeActive)
	}
}

func TestHandleConnSuccessfulHandshakeResultDoesNotCountError(t *testing.T) {
	oldOutput := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(oldOutput) })

	serverConn, clientConn := net.Pipe()
	notifiedConn := &closeNotifyConn{Conn: serverConn, closed: make(chan struct{})}
	beforeErrors := metricValue(t, sshConnectionErrors)
	beforeActive := metricValue(t, activeConnections)
	activeConnections.Inc()
	done := make(chan struct{})
	go func() {
		handleConnWithHandshake(notifiedConn, func(net.Conn) error { return nil })
		close(done)
	}()
	defer clientConn.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleConn did not finish after a successful handshake result")
	}
	if got := metricValue(t, sshConnectionErrors); got != beforeErrors {
		t.Errorf("connection errors = %v, want unchanged %v", got, beforeErrors)
	}
	if got := metricValue(t, activeConnections); got != beforeActive {
		t.Errorf("active connections = %v, want restored value %v", got, beforeActive)
	}
}

func TestMetricsHandlerExportsPrometheusMetrics(t *testing.T) {
	recorder := httptest.NewRecorder()
	metricsHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	for _, name := range []string{
		"fakessh_ssh_connections_total",
		"fakessh_active_connections",
		"fakessh_ssh_connection_errors_total",
		"fakessh_password_auth_attempts_total",
	} {
		if !strings.Contains(recorder.Body.String(), name) {
			t.Errorf("metrics response does not contain %q", name)
		}
	}
}

func testRuntimeDependencies() runtimeDependencies {
	return runtimeDependencies{
		now:      time.Now,
		openFile: os.OpenFile,
		listen: func(string, string) (net.Listener, error) {
			return &stubListener{acceptErr: errors.New("stopped")}, nil
		},
		newServerConfig: func() (*ssh.ServerConfig, error) { return &ssh.ServerConfig{}, nil },
		serveMetrics:    func(string, http.Handler) error { return nil },
		fatalf:          func(string, ...any) {},
	}
}

func metricValue(t *testing.T, metric prometheus.Metric) float64 {
	t.Helper()
	value := &dto.Metric{}
	if err := metric.Write(value); err != nil {
		t.Fatalf("read metric value: %v", err)
	}
	if counter := value.GetCounter(); counter != nil {
		return counter.GetValue()
	}
	if gauge := value.GetGauge(); gauge != nil {
		return gauge.GetValue()
	}
	t.Fatalf("metric has no counter or gauge value: %v", value)
	return 0
}

type stubConnMetadata struct{}

func (stubConnMetadata) User() string          { return "test-user" }
func (stubConnMetadata) SessionID() []byte     { return []byte("session") }
func (stubConnMetadata) ClientVersion() []byte { return []byte("SSH-2.0-test-client") }
func (stubConnMetadata) ServerVersion() []byte { return []byte("SSH-2.0-test-server") }
func (stubConnMetadata) RemoteAddr() net.Addr  { return stubAddr("remote") }
func (stubConnMetadata) LocalAddr() net.Addr   { return stubAddr("local") }

type stubAddr string

func (a stubAddr) Network() string { return "test" }
func (a stubAddr) String() string  { return string(a) }

type stubListener struct {
	conn      net.Conn
	returned  bool
	acceptErr error
	closeOnce sync.Once
	closed    bool
}

func (l *stubListener) Accept() (net.Conn, error) {
	if l.conn != nil && !l.returned {
		l.returned = true
		return l.conn, nil
	}
	return nil, l.acceptErr
}

func (l *stubListener) Close() error {
	l.closeOnce.Do(func() { l.closed = true })
	return nil
}

func (l *stubListener) Addr() net.Addr { return stubAddr("listener") }

type closeNotifyConn struct {
	net.Conn
	closed chan struct{}
	once   sync.Once
}

func (c *closeNotifyConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { close(c.closed) })
	return err
}
