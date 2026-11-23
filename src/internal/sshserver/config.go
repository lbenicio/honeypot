package sshserver

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"fmt"

	"github.com/lbenicio/fakessh/src/internal/auth"
	"golang.org/x/crypto/ssh"
)

var serverVersions = []string{
	"SSH-2.0-OpenSSH_6.6.1p1 Ubuntu-2ubuntu2.3",
	"SSH-2.0-OpenSSH_6.7p1 Debian-5+deb8u3",
	"SSH-2.0-OpenSSH_7.2p2 Ubuntu-4ubuntu2.10",
	"SSH-2.0-OpenSSH_7.4",
	"SSH-2.0-OpenSSH_8.0",
	"SSH-2.0-OpenSSH_8.4p1 Debian-2~bpo10+1",
	"SSH-2.0-OpenSSH_8.4p1 Debian-5+deb11u1",
	"SSH-2.0-OpenSSH_8.9p1 Ubuntu-3ubuntu0.6",
}

func NewConfig(passwordCallback auth.PasswordCallback) (*ssh.ServerConfig, error) {
	return newConfigWithSigner(passwordCallback, newHostSigner)
}

func newConfigWithSigner(passwordCallback auth.PasswordCallback, createSigner func() (ssh.Signer, error)) (*ssh.ServerConfig, error) {
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

func newHostSigner() (ssh.Signer, error) {
	return newHostSignerWithFunctions(
		func() (crypto.Signer, error) { return rsa.GenerateKey(rand.Reader, 2048) },
		ssh.NewSignerFromSigner,
	)
}

func newHostSignerWithFunctions(generateKey func() (crypto.Signer, error), makeSigner func(crypto.Signer) (ssh.Signer, error)) (ssh.Signer, error) {
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
