// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

//go:build windows

package ipc

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/Microsoft/go-winio"

	"github.com/gardenoftheazazil/daemonproc/interfaces"
)

// WindowsIpcListener implements interfaces.IIpcListener for Windows named pipes.
type WindowsIpcListener struct {
	pipePath string
	listener net.Listener
	mu       sync.Mutex
	closed   bool
}

// NewWindowsIpcListener creates a new WindowsIpcListener instance.
func NewWindowsIpcListener(pipePath string) *WindowsIpcListener {
	return &WindowsIpcListener{
		pipePath: pipePath,
	}
}

// Start starts listening on the Windows named pipe.
func (l *WindowsIpcListener) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.listener != nil {
		return fmt.Errorf("listener already started")
	}

	config := &winio.PipeConfig{}
	ln, err := winio.ListenPipe(l.pipePath, config)
	if err != nil {
		return fmt.Errorf("failed to listen on Windows named pipe %s: %w", l.pipePath, err)
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

// Stop stops the Windows named pipe listener.
func (l *WindowsIpcListener) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.listener == nil {
		return nil
	}

	l.closed = true
	err := l.listener.Close()
	l.listener = nil

	return err
}

// Accept waits for and returns the next incoming local connection.
func (l *WindowsIpcListener) Accept() (net.Conn, error) {
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
	return NewWindowsIpcListener(address)
}
