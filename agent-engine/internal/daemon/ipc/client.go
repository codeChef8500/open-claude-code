package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// Client connects to an IPC server and exchanges JSON-line messages.
type Client struct {
	socketPath string
	conn       net.Conn
	mu         sync.Mutex
	scanner    *bufio.Scanner
	handlers   map[string]Handler
	handlerMu  sync.RWMutex
}

// NewClient creates a new IPC client targeting the given socket path.
func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
		handlers:   make(map[string]Handler),
	}
}

// Connect dials the IPC server with a timeout.
func (c *Client) Connect(timeout time.Duration) error {
	conn, err := net.DialTimeout("unix", c.socketPath, timeout)
	if err != nil {
		return fmt.Errorf("ipc connect %s: %w", c.socketPath, err)
	}
	c.conn = conn
	c.scanner = bufio.NewScanner(conn)
	c.scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return nil
}

// Send sends a message to the server.
func (c *Client) Send(msg *Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = c.conn.Write(append(data, '\n'))
	return err
}

// SendAndWait sends a message and waits for a reply with matching ReplyTo.
func (c *Client) SendAndWait(msg *Message, timeout time.Duration) (*Message, error) {
	if msg.ID == "" {
		msg.ID = fmt.Sprintf("req-%d", time.Now().UnixNano())
	}

	if err := c.Send(msg); err != nil {
		return nil, err
	}

	if c.conn != nil {
		_ = c.conn.SetReadDeadline(time.Now().Add(timeout))
		defer func() { _ = c.conn.SetReadDeadline(time.Time{}) }()
	}

	for c.scanner.Scan() {
		var reply Message
		if err := json.Unmarshal(c.scanner.Bytes(), &reply); err != nil {
			continue
		}
		if reply.ReplyTo == msg.ID {
			return &reply, nil
		}
		// Dispatch non-reply messages to handlers
		c.dispatchMessage(&reply)
	}

	if err := c.scanner.Err(); err != nil {
		return nil, fmt.Errorf("read reply: %w", err)
	}
	return nil, fmt.Errorf("connection closed without reply")
}

// OnMessage registers a handler for a message type.
func (c *Client) OnMessage(msgType string, handler Handler) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.handlers[msgType] = handler
}

// ReadLoop reads messages from the server and dispatches to handlers.
// Blocks until the connection is closed or an error occurs.
func (c *Client) ReadLoop() error {
	if c.scanner == nil {
		return fmt.Errorf("not connected")
	}
	for c.scanner.Scan() {
		var msg Message
		if err := json.Unmarshal(c.scanner.Bytes(), &msg); err != nil {
			continue
		}
		c.dispatchMessage(&msg)
	}
	return c.scanner.Err()
}

// Close closes the connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

func (c *Client) dispatchMessage(msg *Message) {
	c.handlerMu.RLock()
	h, ok := c.handlers[msg.Type]
	c.handlerMu.RUnlock()
	if ok && h != nil {
		_ = h(msg)
	}
}
