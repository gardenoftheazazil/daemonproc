// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package interfaces

import "net"

// HandshakeSession represents the cryptographic session established
// after a successful handshake with a peer.
type HandshakeSession struct {
	TargetAddr   *net.UDPAddr
	PeerIdentity []byte // Cross peer identity (e.g., 32-byte Curve25519 public key).

	// CipherSuite defines which algorithm is used
	// (e.g., "Noise_IKpsk2_25519_ChaChaPoly_SHA256").
	// This makes it extremely easy to support other protocols in the future.
	CipherSuite string

	InboundKey  []byte // Key for decrypting incoming packets.
	OutboundKey []byte // Key for encrypting outgoing packets.
}
