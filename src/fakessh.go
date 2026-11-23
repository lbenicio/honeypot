package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/ssh"
)

var (
	errBadPassword = errors.New("permission denied")
	metricsOnce    sync.Once
	sshReady       atomic.Bool
	sshConnections = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "fakessh_ssh_connections_total",
		Help: "Total number of accepted SSH TCP connections.",
	})
	activeConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "fakessh_active_connections",
		Help: "Number of SSH TCP connections currently being handled.",
	})
	sshConnectionErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "fakessh_ssh_connection_errors_total",
		Help: "Total number of SSH connections that ended with an error, including rejected authentication.",
	})
	passwordAttempts = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "fakessh_password_auth_attempts_total",
		Help: "Total number of password authentication attempts.",
	})
	serverVersions = []string{
		"SSH-2.0-OpenSSH_6.6.1p1 Ubuntu-2ubuntu2.3",
		"SSH-2.0-OpenSSH_6.7p1 Debian-5+deb8u3",
		"SSH-2.0-OpenSSH_7.2p2 Ubuntu-4ubuntu2.10",
		"SSH-2.0-OpenSSH_7.4",
		"SSH-2.0-OpenSSH_8.0",
		"SSH-2.0-OpenSSH_8.4p1 Debian-2~bpo10+1",
		"SSH-2.0-OpenSSH_8.4p1 Debian-5+deb11u1",
		"SSH-2.0-OpenSSH_8.9p1 Ubuntu-3ubuntu0.6",
	}
)

type runtimeDependencies struct {
	now             func() time.Time
	openFile        func(string, int, os.FileMode) (*os.File, error)
	listen          func(string, string) (net.Listener, error)
	newServerConfig func() (*ssh.ServerConfig, error)
	serveMetrics    func(string, http.Handler) error
	fatalf          func(string, ...any)
}

var defaultRuntimeDependencies = runtimeDependencies{
	now:             time.Now,
	openFile:        os.OpenFile,
	listen:          net.Listen,
	newServerConfig: newSSHServerConfig,
	serveMetrics:    http.ListenAndServe,
	fatalf:          log.Fatalf,
}

var runApplication = func(args []string, metricsAddr string) error {
	return runApplicationWithDependencies(args, metricsAddr, defaultRuntimeDependencies)
}

var processExit = os.Exit
var healthCheckClient = &http.Client{Timeout: 2 * time.Second}
var runHealthCheck = func() error {
	endpoint, err := healthCheckEndpoint(os.Getenv("PROMETHEUS_ADDR"))
	if err != nil {
		return err
	}
	return checkHealth(healthCheckClient, endpoint)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := runHealthCheck(); err != nil {
			log.Println(err)
			processExit(1)
			return
		}
		return
	}
	if err := runApplication(os.Args[1:], os.Getenv("PROMETHEUS_ADDR")); err != nil {
		log.Println(err)
	}
}

func runApplicationWithDependencies(args []string, metricsAddr string, deps runtimeDependencies) error {
	if metricsAddr == "" {
		metricsAddr = ":2112"
	}

	if len(args) > 0 {
		logPath := fmt.Sprintf("%s/fakessh-%s.log", args[0], deps.now().Format("2006-01-02-15-04-05-000"))
		logFile, err := deps.openFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("failed to open log file %q: %w", logPath, err)
		}
		defer func() {
			if err := logFile.Close(); err != nil {
				log.Printf("Failed to close log file: %v", err)
			}
		}()
		log.SetOutput(logFile)
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	registerMetrics()
	startMetricsServer(metricsAddr, deps.serveMetrics, deps.fatalf)

	serverConfig, err := deps.newServerConfig()
	if err != nil {
		return fmt.Errorf("create SSH server configuration: %w", err)
	}

	listener, err := deps.listen("tcp", ":22")
	if err != nil {
		return fmt.Errorf("failed to listen for SSH connections: %w", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.Printf("Failed to close SSH listener: %v", err)
		}
	}()
	sshReady.Store(true)
	defer sshReady.Store(false)
	serveSSH(listener, serverConfig)
	return nil
}

func registerMetrics() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(sshConnections, activeConnections, sshConnectionErrors, passwordAttempts)
	})
}

func metricsHandler() http.Handler {
	registerMetrics()
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", healthHandler)
	return mux
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	if !sshReady.Load() {
		http.Error(w, "SSH listener is not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func checkHealth(client *http.Client, endpoint string) error {
	response, err := client.Get(endpoint)
	if err != nil {
		return fmt.Errorf("healthcheck request failed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck returned HTTP %d", response.StatusCode)
	}
	return nil
}

func healthCheckEndpoint(metricsAddr string) (string, error) {
	if metricsAddr == "" {
		metricsAddr = ":2112"
	}
	_, port, err := net.SplitHostPort(metricsAddr)
	if err != nil || port == "" {
		return "", fmt.Errorf("invalid Prometheus address %q", metricsAddr)
	}
	return "http://127.0.0.1:" + port + "/healthz", nil
}

func startMetricsServer(addr string, serve func(string, http.Handler) error, fatalf func(string, ...any)) {
	handler := metricsHandler()
	go func() {
		log.Printf("Serving Prometheus metrics on %s/metrics", addr)
		if err := serve(addr, handler); err != nil {
			fatalf("Failed to serve Prometheus metrics: %v", err)
		}
	}()
}

func newSSHServerConfig() (*ssh.ServerConfig, error) {
	return newSSHServerConfigWithSigner(newSSHHostSigner)
}

func newSSHServerConfigWithSigner(createSigner func() (ssh.Signer, error)) (*ssh.ServerConfig, error) {
	signer, err := createSigner()
	if err != nil {
		return nil, fmt.Errorf("create SSH server host key: %w", err)
	}
	serverConfig := &ssh.ServerConfig{
		MaxAuthTries:     6,
		PasswordCallback: passwordCallback,
		ServerVersion:    serverVersions[0],
	}

	serverConfig.AddHostKey(signer)
	return serverConfig, nil
}

func newSSHHostSigner() (ssh.Signer, error) {
	return newSSHHostSignerWithFunctions(
		func() (crypto.Signer, error) { return rsa.GenerateKey(rand.Reader, 2048) },
		ssh.NewSignerFromSigner,
	)
}

func newSSHHostSignerWithFunctions(generateKey func() (crypto.Signer, error), makeSigner func(crypto.Signer) (ssh.Signer, error)) (ssh.Signer, error) {
	privateKey, err := generateKey()
	if err != nil {
		return nil, fmt.Errorf("generate SSH host key: %w", err)
	}
	signer, err := makeSigner(privateKey)
	if err != nil {
		return nil, fmt.Errorf("create SSH host key signer: %w", err)
	}
	return signer, nil
}

func serveSSH(listener net.Listener, serverConfig *ssh.ServerConfig) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Failed to accept:", err)
			return
		}
		sshConnections.Inc()
		activeConnections.Inc()
		go handleConn(conn, serverConfig)
	}
}

var passwordCallbackDelay = 100 * time.Millisecond

func passwordCallback(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	passwordAttempts.Inc()
	log.Println(conn.RemoteAddr(), string(conn.ClientVersion()), conn.User(), string(password))
	time.Sleep(passwordCallbackDelay)
	return nil, errBadPassword
}

func handleConn(conn net.Conn, serverConfig *ssh.ServerConfig) {
	handleConnWithHandshake(conn, func(conn net.Conn) error {
		_, _, _, err := ssh.NewServerConn(conn, serverConfig)
		return err
	})
}

func handleConnWithHandshake(conn net.Conn, handshake func(net.Conn) error) {
	defer func() { _ = conn.Close() }()
	defer activeConnections.Dec()
	log.Println(conn.RemoteAddr())
	if err := handshake(conn); err != nil {
		sshConnectionErrors.Inc()
	}
}
