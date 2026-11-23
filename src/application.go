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
	newLogger       func() *log.Logger
	openFile        func(string, int, os.FileMode) (*os.File, error)
	listen          func(string, string) (net.Listener, error)
	newServerConfig func(auth.PasswordCallback) (*ssh.ServerConfig, error)
	serveMetrics    func(string, http.Handler) error
	fatalf          func(string, ...any)
}

var defaultRuntimeDependencies = runtimeDependencies{
	newLogger:       func() *log.Logger { return log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds) },
	openFile:        os.OpenFile,
	listen:          net.Listen,
	newServerConfig: sshserver.NewConfig,
	serveMetrics:    http.ListenAndServe,
	fatalf:          log.Fatalf,
}

var runApplication = func(metricsAddr, logFile string) error {
	return runApplicationWithDependencies(metricsAddr, logFile, defaultRuntimeDependencies)
}

func runApplicationWithDependencies(metricsAddr, logFile string, deps runtimeDependencies) error {
	if metricsAddr == "" {
		metricsAddr = defaultMetricsAddress
	}

	logger := deps.newLogger()
	if logFile != "" {
		file, err := deps.openFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("failed to open log file %q: %w", logFile, err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				logger.Printf("Failed to close log file: %v", err)
			}
		}()
		logger.SetOutput(file)
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
