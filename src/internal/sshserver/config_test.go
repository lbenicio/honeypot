package sshserver

import (
	"crypto"
	"errors"
	"testing"

	"github.com/lbenicio/fakessh/src/internal/auth"
	"golang.org/x/crypto/ssh"
)

func TestNewConfig(t *testing.T) {
	callback := auth.PasswordCallback(func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) {
		return nil, nil
	})
	config, err := NewConfig(callback)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
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

func TestNewConfigReturnsSignerError(t *testing.T) {
	wantErr := errors.New("host key unavailable")
	_, err := newConfigWithSigner(nil, func() (ssh.Signer, error) { return nil, wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestNewHostSignerReturnsKeyGenerationError(t *testing.T) {
	wantErr := errors.New("random source failed")
	_, err := newHostSignerWithFunctions(
		func() (crypto.Signer, error) { return nil, wantErr },
		func(crypto.Signer) (ssh.Signer, error) { return nil, errors.New("must not be called") },
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestNewHostSignerReturnsSignerError(t *testing.T) {
	wantErr := errors.New("signer creation failed")
	_, err := newHostSignerWithFunctions(
		func() (crypto.Signer, error) { return nil, nil },
		func(crypto.Signer) (ssh.Signer, error) { return nil, wantErr },
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}
