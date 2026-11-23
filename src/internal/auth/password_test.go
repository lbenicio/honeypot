package auth

import (
	"bytes"
	"errors"
	"log"
	"net"
	"strings"
	"testing"
	"time"
)

func TestPasswordCallbackRecordsLogsAndRejects(t *testing.T) {
	var output bytes.Buffer
	logger := log.New(&output, "", 0)
	attempts := 0
	callback := NewPasswordCallback(func() { attempts++ }, logger, 0)
	permissions, err := callback(testMetadata{}, []byte("incorrect"))
	if permissions != nil {
		t.Errorf("permissions = %v, want nil", permissions)
	}
	if !errors.Is(err, ErrBadPassword) {
		t.Errorf("error = %v, want %v", err, ErrBadPassword)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
	for _, expected := range []string{"remote", "SSH-2.0-test-client", "test-user", "incorrect"} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("log %q does not contain %q", output.String(), expected)
		}
	}
}

func TestPasswordCallbackAllowsNilObserverAndLogger(t *testing.T) {
	defaultLogger := log.Default()
	oldWriter := defaultLogger.Writer()
	defaultLogger.SetOutput(bytes.NewBuffer(nil))
	t.Cleanup(func() { defaultLogger.SetOutput(oldWriter) })

	callback := NewPasswordCallback(nil, nil, time.Nanosecond)
	_, err := callback(testMetadata{}, []byte("bad"))
	if !errors.Is(err, ErrBadPassword) {
		t.Errorf("error = %v, want %v", err, ErrBadPassword)
	}
}

type testMetadata struct{}

func (testMetadata) User() string          { return "test-user" }
func (testMetadata) SessionID() []byte     { return []byte("session") }
func (testMetadata) ClientVersion() []byte { return []byte("SSH-2.0-test-client") }
func (testMetadata) ServerVersion() []byte { return []byte("SSH-2.0-test-server") }
func (testMetadata) RemoteAddr() net.Addr  { return testAddress("remote") }
func (testMetadata) LocalAddr() net.Addr   { return testAddress("local") }

type testAddress string

func (a testAddress) Network() string { return "test" }
func (a testAddress) String() string  { return string(a) }
