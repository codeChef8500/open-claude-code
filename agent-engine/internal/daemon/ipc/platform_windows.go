//go:build windows

package ipc

import (
	"net"
	"time"
)

// listenPlatform creates a listener. On Windows we use Unix domain sockets
// which are supported since Windows 10 1803+ / Go 1.12+.
// For broader compatibility, this could be replaced with Named Pipes
// via github.com/Microsoft/go-winio.
func listenPlatform(socketPath string) (net.Listener, error) {
	return net.Listen("unix", socketPath)
}

// dialPlatform connects to a socket. Uses Unix domain sockets on modern Windows.
func dialPlatform(socketPath string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", socketPath, timeout)
}
