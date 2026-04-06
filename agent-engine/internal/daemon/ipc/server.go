package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// ─── Message types ──────────────────────────────────────────────────────────

// Message is the IPC envelope exchanged between supervisor and worker,
// or between CLI and daemon. Aligned with claude-code-main messaging socket.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	ReplyTo string          `json:"reply_to,omitempty"`
	ID      string          `json:"id,omitempty"`
}

// Handler processes an incoming IPC message and returns an optional reply.
type Handler func(msg *Message) *Message

// ─── Server ─────────────────────────────────────────────────────────────────

// Server listens on a Unix domain socket (or named pipe on Windows)
// for JSON-line messages. Each connection is handled in its own goroutine.
type Server struct {
	socketPath string
	listener   net.Listener
	handler    Handler
	mu         sync.Mutex
	conns      map[net.Conn]struct{}
}

// NewServer creates a new IPC server at the given socket path.
func NewServer(socketPath string, handler Handler) *Server {
	return &Server{
		socketPath: socketPath,
		handler:    handler,
		conns:      make(map[net.Conn]struct{}),
	}
}

// SocketPath returns the path the server is listening on.
func (s *Server) SocketPath() string {
	return s.socketPath
}

// Listen starts accepting connections. Blocks until ctx is cancelled.
func (s *Server) Listen(ctx context.Context) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0700); err != nil {
		return fmt.Errorf("create socket dir: %w", err)
	}

	// Remove stale socket file
	_ = os.Remove(s.socketPath)

	ln, err := listenPlatform(s.socketPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.socketPath, err)
	}
	s.listener = ln

	slog.Info("ipc: server listening", slog.String("path", s.socketPath))

	// Accept loop
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown
			}
			slog.Warn("ipc: accept error", slog.Any("err", err))
			continue
		}

		s.mu.Lock()
		s.conns[conn] = struct{}{}
		s.mu.Unlock()

		go s.handleConn(ctx, conn)
	}
}

// Close shuts down the server and all connections.
func (s *Server) Close() error {
	s.mu.Lock()
	for c := range s.conns {
		_ = c.Close()
	}
	s.conns = make(map[net.Conn]struct{})
	s.mu.Unlock()

	if s.listener != nil {
		_ = s.listener.Close()
	}
	_ = os.Remove(s.socketPath)
	return nil
}

// Broadcast sends a message to all connected clients.
func (s *Server) Broadcast(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	line := append(data, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.conns {
		_, _ = c.Write(line)
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer func() {
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
		_ = conn.Close()
	}()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}

		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			slog.Debug("ipc: invalid message", slog.Any("err", err))
			continue
		}

		if s.handler != nil {
			reply := s.handler(&msg)
			if reply != nil {
				data, _ := json.Marshal(reply)
				_, _ = conn.Write(append(data, '\n'))
			}
		}
	}
}
