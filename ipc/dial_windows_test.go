// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

//go:build windows

package ipc

import (
	"net"

	"github.com/Microsoft/go-winio"
)

func dialTestAddress(address string) (net.Conn, error) {
	return winio.DialPipe(address, nil)
}
