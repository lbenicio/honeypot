package sshserver

import (
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestServeAcceptsAndHandlesConnection(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	notifiedConn := &closeNotifyConn{Conn: serverConn, closed: make(chan struct{})}
	listener := &testListener{conn: notifiedConn, acceptErr: errors.New("listener stopped")}
	metrics := &testMetrics{}
	Serve(listener, &ssh.ServerConfig{}, metrics, log.New(io.Discard, "", 0))
	_ = clientConn.Close()
	select {
	case <-notifiedConn.closed:
	case <-time.After(time.Second):
		t.Fatal("SSH connection was not closed")
	}
	if got := metrics.snapshot(); got != [4]int{1, 1, 1, 1} {
		t.Errorf("metrics counts = %v, want accepted, active, finished, failed = [1 1 1 1]", got)
	}
}

func TestHandleConnectionSuccessfulHandshakeDoesNotRecordFailure(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	notifiedConn := &closeNotifyConn{Conn: serverConn, closed: make(chan struct{})}
	metrics := &testMetrics{}
	metrics.ConnectionStarted()
	done := make(chan struct{})
	go func() {
		handleConnectionWithHandshake(notifiedConn, metrics, log.New(io.Discard, "", 0), func(net.Conn) error { return nil })
		close(done)
	}()
	defer func() { _ = clientConn.Close() }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("connection handler did not finish")
	}
	if got := metrics.snapshot(); got != [4]int{0, 1, 1, 0} {
		t.Errorf("metrics counts = %v, want accepted, active, finished, failed = [0 1 1 0]", got)
	}
}

type testMetrics struct {
	mu       sync.Mutex
	accepted int
	started  int
	finished int
	failed   int
}

func (m *testMetrics) ConnectionAccepted() { m.mu.Lock(); m.accepted++; m.mu.Unlock() }
func (m *testMetrics) ConnectionStarted()  { m.mu.Lock(); m.started++; m.mu.Unlock() }
func (m *testMetrics) ConnectionFinished() { m.mu.Lock(); m.finished++; m.mu.Unlock() }
func (m *testMetrics) ConnectionFailed()   { m.mu.Lock(); m.failed++; m.mu.Unlock() }

func (m *testMetrics) snapshot() [4]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return [4]int{m.accepted, m.started, m.finished, m.failed}
}

type testListener struct {
	conn      net.Conn
	returned  bool
	acceptErr error
}

func (l *testListener) Accept() (net.Conn, error) {
	if l.conn != nil && !l.returned {
		l.returned = true
		return l.conn, nil
	}
	return nil, l.acceptErr
}

func (l *testListener) Close() error   { return nil }
func (l *testListener) Addr() net.Addr { return testAddress("listener") }

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

type testAddress string

func (a testAddress) Network() string { return "test" }
func (a testAddress) String() string  { return string(a) }
