package auth

import (
	"errors"
	"log"
	"time"

	"golang.org/x/crypto/ssh"
)

var ErrBadPassword = errors.New("permission denied")

type PasswordCallback func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error)

// NewPasswordCallback records and rejects every password attempt.
func NewPasswordCallback(recordAttempt func(), logger *log.Logger, delay time.Duration) PasswordCallback {
	if logger == nil {
		logger = log.Default()
	}

	return func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if recordAttempt != nil {
			recordAttempt()
		}
		logger.Println(conn.RemoteAddr(), string(conn.ClientVersion()), conn.User(), string(password))
		time.Sleep(delay)
		return nil, ErrBadPassword
	}
}
