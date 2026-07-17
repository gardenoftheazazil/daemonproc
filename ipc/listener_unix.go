// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

//go:build !windows && !android

package ipc

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/gardenoftheazazil/daemonproc/interfaces"
)

// UnixIpcListener implements interfaces.IIpcListener for Unix-like systems.
type UnixIpcListener struct {
	socketPath string
	listener   net.Listener
	mu         sync.Mutex
	closed     bool
}

// NewUnixIpcListener creates a new UnixIpcListener instance.
func NewUnixIpcListener(socketPath string) *UnixIpcListener {
	return &UnixIpcListener{
		socketPath: socketPath,
	}
}

// Start starts listening on the Unix domain socket.
func (l *UnixIpcListener) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.listener != nil {
		return fmt.Errorf("listener already started")
	}

	// Remove existing socket file if it exists.
	if err := os.Remove(l.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clean up existing socket file %s: %w", l.socketPath, err)
	}

	// Listen on Unix domain socket.
	// Since Go 1.20+, we can pass the context to Dial/Listen by wrapping, but net.Listen
	// doesn't directly accept context. We listen and close on context cancellation if needed.
	// However, IIpcListener has Start(ctx) which typically just starts listening.
	ln, err := net.Listen("unix", l.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on Unix domain socket %s: %w", l.socketPath, err)
	}

	l.listener = ln
	l.closed = false

	// Handle context cancellation to stop the listener.
	go func() {
		<-ctx.Done()
		_ = l.Stop()
	}()

	return nil
}

// Stop stops the Unix domain socket listener and removes the socket file.
func (l *UnixIpcListener) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.listener == nil {
		return nil
	}

	l.closed = true
	err := l.listener.Close()
	l.listener = nil

	// Clean up socket file.
	_ = os.Remove(l.socketPath)

	return err
}

// Accept waits for and returns the next incoming local connection.
func (l *UnixIpcListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	ln := l.listener
	closed := l.closed
	l.mu.Unlock()

	if closed || ln == nil {
		return nil, fmt.Errorf("listener is closed")
	}

	conn, err := ln.Accept()
	if err != nil {
		return nil, fmt.Errorf("failed to accept local connection: %w", err)
	}
	return conn, nil
}

func newPlatformListener(address string) interfaces.IIpcListener {
	return NewUnixIpcListener(address)
}
