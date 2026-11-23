package main

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestMainHealthcheckMode(t *testing.T) {
	oldArgs := os.Args
	oldRun := runHealthCheck
	oldExit := processExit
	oldOutput := log.Writer()
	t.Cleanup(func() {
		os.Args = oldArgs
		runHealthCheck = oldRun
		processExit = oldExit
		log.SetOutput(oldOutput)
	})
	os.Args = []string{"fakessh", "healthcheck"}
	log.SetOutput(io.Discard)

	called := false
	exitCode := 0
	runHealthCheck = func() error {
		called = true
		return nil
	}
	processExit = func(code int) { exitCode = code }
	main()
	if !called {
		t.Fatal("healthcheck mode did not run the health check")
	}
	if exitCode != 0 {
		t.Fatalf("exit code = %d after successful health check, want no exit", exitCode)
	}

	wantErr := errors.New("health endpoint unavailable")
	var output bytes.Buffer
	log.SetOutput(&output)
	runHealthCheck = func() error { return wantErr }
	main()
	if exitCode != 1 {
		t.Errorf("exit code = %d after failed health check, want 1", exitCode)
	}
	if !strings.Contains(output.String(), wantErr.Error()) {
		t.Errorf("health check error was not logged: %q", output.String())
	}
}

func TestMainStartupMode(t *testing.T) {
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
	called := 0
	runApplication = func(args []string, metricsAddr string) error {
		called++
		if len(args) != 1 || args[0] != "/tmp/logs" {
			t.Errorf("runApplication args = %v, want [/tmp/logs]", args)
		}
		if metricsAddr != "127.0.0.1:9211" {
			t.Errorf("metrics address = %q, want 127.0.0.1:9211", metricsAddr)
		}
		if called == 1 {
			return errors.New("startup failed")
		}
		return nil
	}

	main()
	if !strings.Contains(output.String(), "startup failed") {
		t.Fatalf("main did not report startup error: %q", output.String())
	}
	main()
	if called != 2 {
		t.Errorf("runApplication calls = %d, want 2", called)
	}
}

func TestRunHealthCheckUsesConfiguredClient(t *testing.T) {
	oldClient := healthCheckClient
	oldRun := runHealthCheck
	t.Cleanup(func() {
		healthCheckClient = oldClient
		runHealthCheck = oldRun
	})
	t.Setenv("PROMETHEUS_ADDR", "")
	healthCheckClient = &http.Client{Transport: testRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "http://127.0.0.1:2112/healthz" {
			t.Errorf("healthcheck URL = %q, want http://127.0.0.1:2112/healthz", request.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}
	if err := runHealthCheck(); err != nil {
		t.Fatalf("runHealthCheck() error = %v, want nil", err)
	}
}

type testRoundTripper func(*http.Request) (*http.Response, error)

func (f testRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
