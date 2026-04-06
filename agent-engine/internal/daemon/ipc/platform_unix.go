//go:build !windows

package ipc

import (
	"net"
	"time"
)

// listenPlatform creates a Unix domain socket listener.
func listenPlatform(socketPath string) (net.Listener, error) {
	return net.Listen("unix", socketPath)
}

// dialPlatform connects to a Unix domain socket.
func dialPlatform(socketPath string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", socketPath, timeout)
}
