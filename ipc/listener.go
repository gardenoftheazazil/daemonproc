// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

// Package ipc is responsible for managing local application and socket sessions, and communication management.
package ipc

import (
	"github.com/gardenoftheazazil/daemonproc/interfaces"
)

// NewIpcListener returns the platform-specific IIpcListener implementation.
// On Windows, the address is a named pipe path (e.g., `\\.\pipe\gota`).
// On Unix-like systems, the address is a socket file path (e.g., `/tmp/gota.sock`).
func NewIpcListener(address string) interfaces.IIpcListener {
	return newPlatformListener(address)
}
