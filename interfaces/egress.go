// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package interfaces

import "net"

// IEgress represents the interface for sending data to a target address using a specified session.
type IEgress interface {
	// Send data to the specified target address using the provided session information.
	// The session information is used to encrypt the data before sending it.
	// Returns an error if the send operation fails.
	Send(targetAddr *net.UDPAddr, portListener IUdpPortListener, session HandshakeSession, data []byte) error
}
