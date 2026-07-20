// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package interfaces

import (
	"context"
	"net"
)

// DID is dispatch id data type.
type DID uint32

// IIpcListener manages the platform-specific local inter-process communication
// layers, abstracting Unix domain sockets on Unix-like systems and bidirectional
// Named Pipes on Windows.
type IIpcListener interface {
	// Start initializes the underlying local IPC socket or pipe and begins listening.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the local IPC listener and cleans up resources.
	Stop() error

	// Accept waits for and returns the next incoming local application connection.
	Accept() (net.Conn, error)
}

// ControlCallback defines the signature for handling raw local IPC frames.
type ControlCallback func(did DID, opcode uint16, payload []byte)

// IIpcSessionManager coordinates the lifecycle of local application connections
// and maps them to their respective Dispatch IDs (DIDs) for multiplexing.
type IIpcSessionManager interface {
	// RegisterSession assigns a unique DID to a new local connection and tracks it.
	RegisterSession(conn net.Conn) (DID, error)

	// UnregisterSession removes a session by its DID and closes the connection.
	UnregisterSession(did DID) error

	// SendToLocal routes control or network payloads to the specific local application.
	SendToLocal(did DID, opcode, statusCode uint16, data []byte) error

	// SetControlCallback registers a callback to receive incoming raw local IPC frames.
	SetControlCallback(cb ControlCallback)
}
