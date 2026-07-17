// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package interfaces

import (
	"context"
	"net"
)

// HandshakeFailErr represents an error type for handshake failures.
type HandshakeFailErr error

// NewHandshakeFailErr creates a new HandshakeFailErr with the given reason.
func NewHandshakeFailErr(reason string) HandshakeFailErr {
	return &handshakeFailError{reason: reason}
}

type handshakeFailError struct {
	reason string
}

func (e *handshakeFailError) Error() string {
	return e.reason
}

// IHandshakerSubscriber defines the callbacks for handshake success or failure.
type IHandshakerSubscriber interface {
	// OnHandshakeSuccess is called when a handshake is successfully completed.
	OnHandshakeSuccess(session *HandshakeSession)

	// OnHandshakeFailure is called when a handshake fails.
	OnHandshakeFailure(targetAddr *net.UDPAddr, err error)
}

// IHandshaker defines the methods required to initiate a handshake.
type IHandshaker interface {
	// StartHandshake begins the handshake process with a target peer.
	StartHandshake(
		ctx context.Context,
		udpPortListener IUdpPortListener,
		targetAddr *net.UDPAddr,
		inviteKey string,
		subscriber IHandshakerSubscriber,
	) error
}
