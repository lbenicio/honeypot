package main

import (
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

	"github.com/lbenicio/fakessh/src/internal/auth"
	"golang.org/x/crypto/ssh"
)

func TestRunApplicationUsesDefaultDependencies(t *testing.T) {
	oldDependencies := defaultRuntimeDependencies
	t.Cleanup(func() { defaultRuntimeDependencies = oldDependencies })
	wantErr := errors.New("configuration unavailable")
	defaultRuntimeDependencies = testRuntimeDependencies()
	defaultRuntimeDependencies.newServerConfig = func(auth.PasswordCallback) (*ssh.ServerConfig, error) {
		return nil, wantErr
	}

	err := runApplication(":0", "")
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestRunApplicationWithDependencies(t *testing.T) {
	t.Run("defaults metrics address and writes to the configured log file", func(t *testing.T) {
		logDir := t.TempDir()
		logFile := filepath.Join(logDir, "fakessh.log")
		serverConn, clientConn := net.Pipe()
		notifiedConn := &closeNotifyConn{Conn: serverConn, closed: make(chan struct{})}
		listener := &stubListener{conn: notifiedConn, acceptErr: errors.New("listener stopped")}
		metricsResult := make(chan struct {
			addr string
			body string
		}, 1)
		deps := testRuntimeDependencies()
		deps.openFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
			if name != logFile {
				t.Errorf("log file path = %q, want %q", name, logFile)
			}
			return os.OpenFile(name, flag, perm)
		}
		deps.listen = func(network, address string) (net.Listener, error) {
			if network != "tcp" || address != ":22" {
				t.Errorf("listen(%q, %q), want tcp :22", network, address)
			}
			return listener, nil
		}
		deps.newServerConfig = func(callback auth.PasswordCallback) (*ssh.ServerConfig, error) {
			if callback == nil {
				t.Error("password callback is nil")
			}
			return &ssh.ServerConfig{PasswordCallback: callback}, nil
		}
		deps.serveMetrics = func(addr string, handler http.Handler) error {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
			metricsResult <- struct {
				addr string
				body string
			}{addr: addr, body: recorder.Body.String()}
			return nil
		}

		if err := runApplicationWithDependencies("", logFile, deps); err != nil {
			t.Fatalf("runApplicationWithDependencies() error = %v", err)
		}
		_ = clientConn.Close()
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
		if _, err := os.Stat(logFile); err != nil {
			t.Errorf("log file was not created: %v", err)
		}
		if !listener.closed {
			t.Error("SSH listener was not closed")
		}
	})

	t.Run("returns configuration error and uses custom metrics address", func(t *testing.T) {
		wantErr := errors.New("bad SSH config")
		metricsAddress := make(chan string, 1)
		deps := testRuntimeDependencies()
		deps.newServerConfig = func(auth.PasswordCallback) (*ssh.ServerConfig, error) { return nil, wantErr }
		deps.serveMetrics = func(addr string, _ http.Handler) error {
			metricsAddress <- addr
			return nil
		}
		err := runApplicationWithDependencies("127.0.0.1:9211", "", deps)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want wrapped %v", err, wantErr)
		}
		select {
		case got := <-metricsAddress:
			if got != "127.0.0.1:9211" {
				t.Errorf("metrics address = %q, want 127.0.0.1:9211", got)
			}
		case <-time.After(time.Second):
			t.Fatal("metrics server callback was not invoked")
		}
	})

	t.Run("returns listener error", func(t *testing.T) {
		wantErr := errors.New("port unavailable")
		deps := testRuntimeDependencies()
		deps.listen = func(string, string) (net.Listener, error) { return nil, wantErr }
		err := runApplicationWithDependencies(":2112", "", deps)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want wrapped %v", err, wantErr)
		}
	})

	t.Run("returns log file error", func(t *testing.T) {
		wantErr := errors.New("disk unavailable")
		deps := testRuntimeDependencies()
		deps.openFile = func(string, int, os.FileMode) (*os.File, error) { return nil, wantErr }
		err := runApplicationWithDependencies(":2112", "/logs/fakessh.log", deps)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want wrapped %v", err, wantErr)
		}
	})

	t.Run("logs log file close error", func(t *testing.T) {
		deps := testRuntimeDependencies()
		deps.openFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
			file, err := os.OpenFile(name, flag, perm)
			if err != nil {
				return nil, err
			}
			_ = file.Close()
			return file, nil
		}
		wantErr := errors.New("stop after log setup")
		deps.newServerConfig = func(auth.PasswordCallback) (*ssh.ServerConfig, error) { return nil, wantErr }
		err := runApplicationWithDependencies(":2112", filepath.Join(t.TempDir(), "fakessh.log"), deps)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want wrapped %v", err, wantErr)
		}
	})

	t.Run("logs listener close error", func(t *testing.T) {
		deps := testRuntimeDependencies()
		listener := &stubListener{acceptErr: errors.New("stop accepting"), closeErr: errors.New("close failed")}
		deps.listen = func(string, string) (net.Listener, error) { return listener, nil }
		if err := runApplicationWithDependencies(":2112", "", deps); err != nil {
			t.Fatalf("runApplicationWithDependencies() error = %v, want nil", err)
		}
		if !listener.closed {
			t.Error("SSH listener was not closed")
		}
	})
}

func TestStartMetricsServerReportsErrors(t *testing.T) {
	logger := testLogger()
	finished := make(chan struct{}, 1)
	startMetricsServer("127.0.0.1:0", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		func(addr string, handler http.Handler) error {
			if addr != "127.0.0.1:0" || handler == nil {
				t.Errorf("metrics server received address %q or nil handler", addr)
			}
			return errors.New("serve failed")
		},
		func(format string, args ...any) {
			logger.Printf(format, args...)
			finished <- struct{}{}
		}, logger)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("metrics server fatal callback was not invoked")
	}
}

func testRuntimeDependencies() runtimeDependencies {
	return runtimeDependencies{
		newLogger: testLogger,
		openFile:  os.OpenFile,
		listen: func(string, string) (net.Listener, error) {
			return &stubListener{acceptErr: errors.New("stopped")}, nil
		},
		newServerConfig: func(callback auth.PasswordCallback) (*ssh.ServerConfig, error) {
			return &ssh.ServerConfig{PasswordCallback: callback}, nil
		},
		serveMetrics: func(string, http.Handler) error { return nil },
		fatalf:       func(string, ...any) {},
	}
}

func testLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

type stubListener struct {
	conn      net.Conn
	returned  bool
	acceptErr error
	closeErr  error
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
	return l.closeErr
}

func (l *stubListener) Addr() net.Addr { return testAddr("listener") }

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

type testAddr string

func (a testAddr) Network() string { return "test" }
func (a testAddr) String() string  { return string(a) }
