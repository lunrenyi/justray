// Package server is the RPC transport: it owns the socket and dispatches
// requests to connection.Service and subscription.Service. It holds no VPN
// state of its own.
package server

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/charmbracelet/log"

	"github.com/luynrs/justray/internal/daemon/connection"
	"github.com/luynrs/justray/internal/daemon/platform/lock"
	"github.com/luynrs/justray/internal/daemon/subscription"
)

const idle = 60 * time.Second

type Server struct {
	log  *log.Logger
	conn *connection.Service
	subs *subscription.Service
}

func New(logger *log.Logger, conn *connection.Service, subs *subscription.Service) *Server {
	return &Server{log: logger, conn: conn, subs: subs}
}

func Listen(socket string) (net.Listener, error) {
	unlock, err := lock.File(socket + ".lock")
	if err != nil {
		return nil, err
	}
	defer unlock()

	if conn, err := net.DialTimeout("unix", socket, time.Second); err == nil {
		conn.Close()
		return nil, fmt.Errorf("another justrayd is already listening on %s", socket)
	}
	os.Remove(socket)

	ln, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

func (s *Server) Serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handle(conn)
	}
}

func (s *Server) Restore()  { s.conn.Restore() }
func (s *Server) Shutdown() { s.conn.Shutdown() }
