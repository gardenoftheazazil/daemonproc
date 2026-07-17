// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package interfaces

import (
	"context"
	"net"
)

type natPunchFailErr error

// NewNatPunchFailErr creates a new natPunchFailErr with the given reason.
func NewNatPunchFailErr(reason string) natPunchFailErr {
	return &natPunchFailError{reason: reason}
}

type natPunchFailError struct {
	reason string
}

func (e *natPunchFailError) Error() string {
	return e.reason
}

// INatPunchSubscriber defines the callbacks for NAT punching operations.
type INatPunchSubscriber interface {
	// OnNatPunchSuccess is called when the NAT punching operation succeeds.
	OnNatPunchSuccess(targetAddr *net.UDPAddr)

	// OnNatPunchFailure is called when the NAT punching operation fails.
	OnNatPunchFailure(targetAddr *net.UDPAddr, err error)
}

// INatPuncher is an interface that defines the methods for performing
// NAT punching operations. When the operation ends, if it succeeded then
// the natpuncher calls the handshaker to start a handshake; otherwise it
// returns an error with the reason of failure. The handshaker will then
// handle the handshake process with the target peer.
type INatPuncher interface {
	// StartPunch begins the NAT punching procedure.
	StartPunch(
		ctx context.Context,
		udpPortListener IUdpPortListener,
		targetAddr *net.UDPAddr,
		inviteKey string,
		subscriber INatPunchSubscriber,
	) error
}
