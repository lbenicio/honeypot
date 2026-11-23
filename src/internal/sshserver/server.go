package sshserver

import (
	"log"
	"net"

	"golang.org/x/crypto/ssh"
)

type Metrics interface {
	ConnectionAccepted()
	ConnectionStarted()
	ConnectionFinished()
	ConnectionFailed()
}

func Serve(listener net.Listener, serverConfig *ssh.ServerConfig, metrics Metrics, logger *log.Logger) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Println("Failed to accept:", err)
			return
		}
		metrics.ConnectionAccepted()
		metrics.ConnectionStarted()
		go handleConnection(conn, serverConfig, metrics, logger)
	}
}

func handleConnection(conn net.Conn, serverConfig *ssh.ServerConfig, metrics Metrics, logger *log.Logger) {
	handleConnectionWithHandshake(conn, metrics, logger, func(conn net.Conn) error {
		_, _, _, err := ssh.NewServerConn(conn, serverConfig)
		return err
	})
}

func handleConnectionWithHandshake(conn net.Conn, metrics Metrics, logger *log.Logger, handshake func(net.Conn) error) {
	defer func() { _ = conn.Close() }()
	defer metrics.ConnectionFinished()
	logger.Println(conn.RemoteAddr())
	if err := handshake(conn); err != nil {
		metrics.ConnectionFailed()
	}
}
