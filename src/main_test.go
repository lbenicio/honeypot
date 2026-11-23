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
	oldRunAt := runHealthCheckAt
	oldExit := processExit
	oldOutput := log.Writer()
	t.Cleanup(func() {
		os.Args = oldArgs
		runHealthCheck = oldRun
		runHealthCheckAt = oldRunAt
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

func TestMainHealthcheckUsesMetricsFlag(t *testing.T) {
	oldArgs := os.Args
	oldRunAt := runHealthCheckAt
	oldExit := processExit
	t.Cleanup(func() {
		os.Args = oldArgs
		runHealthCheckAt = oldRunAt
		processExit = oldExit
	})
	os.Args = []string{"fakessh", "healthcheck", "--metrics", "127.0.0.1:9211"}
	called := false
	runHealthCheckAt = func(addr string) error {
		called = true
		if addr != "127.0.0.1:9211" {
			t.Errorf("healthcheck metrics address = %q, want 127.0.0.1:9211", addr)
		}
		return nil
	}
	processExit = func(code int) { t.Errorf("processExit(%d) after successful healthcheck", code) }
	main()
	if !called {
		t.Fatal("healthcheck did not use the configured metrics address")
	}
}

func TestMainHealthcheckHelpAndErrors(t *testing.T) {
	oldArgs := os.Args
	oldRunAt := runHealthCheckAt
	oldExit := processExit
	oldOutput := log.Writer()
	oldCLIOutput := cliOutput
	oldCLIErrorOutput := cliErrorOutput
	t.Cleanup(func() {
		os.Args = oldArgs
		runHealthCheckAt = oldRunAt
		processExit = oldExit
		cliOutput = oldCLIOutput
		cliErrorOutput = oldCLIErrorOutput
		log.SetOutput(oldOutput)
	})
	var stdout, stderr, logOutput bytes.Buffer
	cliOutput = &stdout
	cliErrorOutput = &stderr
	log.SetOutput(&logOutput)
	exitCode := 0
	processExit = func(code int) { exitCode = code }

	os.Args = []string{"fakessh", "healthcheck", "--help"}
	runHealthCheckAt = func(string) error { t.Error("healthcheck ran while printing help"); return nil }
	main()
	if !strings.Contains(stdout.String(), "--metrics") {
		t.Errorf("healthcheck help did not print CLI options: %q", stdout.String())
	}
	if exitCode != 0 {
		t.Errorf("exit code after healthcheck help = %d, want 0", exitCode)
	}

	os.Args = []string{"fakessh", "healthcheck", "--unknown"}
	main()
	if exitCode != 2 {
		t.Errorf("exit code for invalid healthcheck flag = %d, want 2", exitCode)
	}

	wantErr := errors.New("metrics endpoint unavailable")
	runHealthCheckAt = func(string) error { return wantErr }
	os.Args = []string{"fakessh", "healthcheck", "--metrics", "127.0.0.1:9211"}
	main()
	if exitCode != 1 {
		t.Errorf("exit code for failed healthcheck = %d, want 1", exitCode)
	}
	if !strings.Contains(logOutput.String(), wantErr.Error()) {
		t.Errorf("healthcheck error was not logged: %q", logOutput.String())
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
	os.Args = []string{"fakessh", "--log-file", "/tmp/logs/fakessh.log"}
	t.Setenv("PROMETHEUS_ADDR", "127.0.0.1:9211")
	var output bytes.Buffer
	log.SetOutput(&output)
	called := 0
	runApplication = func(metricsAddr, logFile string) error {
		called++
		if logFile != "/tmp/logs/fakessh.log" {
			t.Errorf("log file = %q, want /tmp/logs/fakessh.log", logFile)
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

func TestMainHelpAndInvalidFlags(t *testing.T) {
	oldArgs := os.Args
	oldRun := runApplication
	oldExit := processExit
	oldOutput := log.Writer()
	oldCLIOutput := cliOutput
	oldCLIErrorOutput := cliErrorOutput
	t.Cleanup(func() {
		os.Args = oldArgs
		runApplication = oldRun
		processExit = oldExit
		cliOutput = oldCLIOutput
		cliErrorOutput = oldCLIErrorOutput
		log.SetOutput(oldOutput)
	})
	var stdout, stderr, logOutput bytes.Buffer
	cliOutput = &stdout
	cliErrorOutput = &stderr
	log.SetOutput(&logOutput)
	called := false
	runApplication = func(string, string) error {
		called = true
		return nil
	}

	os.Args = []string{"fakessh", "--help"}
	main()
	for _, want := range []string{"Usage:", "--metrics", "--log-file", "healthcheck", "Prometheus metrics"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("help output is missing %q: %q", want, stdout.String())
		}
	}
	if called {
		t.Fatal("application started while printing help")
	}

	exitCode := 0
	processExit = func(code int) { exitCode = code }
	os.Args = []string{"fakessh", "--unknown"}
	main()
	if exitCode != 2 {
		t.Errorf("exit code for invalid flag = %d, want 2", exitCode)
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Errorf("invalid flag error was not written to stderr: %q", stderr.String())
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
