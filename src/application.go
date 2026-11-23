package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/lbenicio/fakessh/src/internal/auth"
	"github.com/lbenicio/fakessh/src/internal/metrics"
	"github.com/lbenicio/fakessh/src/internal/sshserver"
	"golang.org/x/crypto/ssh"
)

type runtimeDependencies struct {
	now             func() time.Time
	newLogger       func() *log.Logger
	openFile        func(string, int, os.FileMode) (*os.File, error)
	listen          func(string, string) (net.Listener, error)
	newServerConfig func(auth.PasswordCallback) (*ssh.ServerConfig, error)
	serveMetrics    func(string, http.Handler) error
	fatalf          func(string, ...any)
}

var defaultRuntimeDependencies = runtimeDependencies{
	now:             time.Now,
	newLogger:       func() *log.Logger { return log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds) },
	openFile:        os.OpenFile,
	listen:          net.Listen,
	newServerConfig: sshserver.NewConfig,
	serveMetrics:    http.ListenAndServe,
	fatalf:          log.Fatalf,
}

var runApplication = func(args []string, metricsAddr string) error {
	return runApplicationWithDependencies(args, metricsAddr, defaultRuntimeDependencies)
}

func runApplicationWithDependencies(args []string, metricsAddr string, deps runtimeDependencies) error {
	if metricsAddr == "" {
		metricsAddr = ":2112"
	}

	logger := deps.newLogger()
	if len(args) > 0 {
		logPath := logFilePath(args[0], deps.now())
		logFile, err := deps.openFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("failed to open log file %q: %w", logPath, err)
		}
		defer func() {
			if err := logFile.Close(); err != nil {
				logger.Printf("Failed to close log file: %v", err)
			}
		}()
		logger.SetOutput(logFile)
	}

	appMetrics := metrics.New()
	startMetricsServer(metricsAddr, appMetrics.Handler(), deps.serveMetrics, deps.fatalf, logger)

	passwordCallback := auth.NewPasswordCallback(appMetrics.PasswordAttempted, logger, 100*time.Millisecond)
	serverConfig, err := deps.newServerConfig(passwordCallback)
	if err != nil {
		return fmt.Errorf("create SSH server configuration: %w", err)
	}

	listener, err := deps.listen("tcp", ":22")
	if err != nil {
		return fmt.Errorf("failed to listen for SSH connections: %w", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			logger.Printf("Failed to close SSH listener: %v", err)
		}
	}()
	appMetrics.SetSSHReady(true)
	defer appMetrics.SetSSHReady(false)
	sshserver.Serve(listener, serverConfig, appMetrics, logger)
	return nil
}

func startMetricsServer(addr string, handler http.Handler, serve func(string, http.Handler) error, fatalf func(string, ...any), logger *log.Logger) {
	go func() {
		logger.Printf("Serving Prometheus metrics on %s/metrics", addr)
		if err := serve(addr, handler); err != nil {
			fatalf("Failed to serve Prometheus metrics: %v", err)
		}
	}()
}
