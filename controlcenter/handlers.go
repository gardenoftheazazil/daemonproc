// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package controlcenter

import (
	"encoding/binary"

	"github.com/gardenoftheazazil/daemonproc/interfaces"
	"github.com/gardenoftheazazil/daemonproc/invitekey"
)

// Standard Opcodes for ABI Subsystems.
const (
	// OpcodeEgressRoute identifies the system call to route outbound application data to the P2P egress network.
	OpcodeEgressRoute uint16 = 0x0001

	// OpcodeGetInviteKey identifies the system call to generate an authenticated invite key (Opcode Group 0x0200).
	OpcodeGetInviteKey uint16 = 0x0201
)

// MakeEgressRouteHandler constructs a SyscallHandler for the OpcodeEgressRoute system call.
// ABI parameter encoding for array/payload: [DataLength uint16][Data Bytes].
func MakeEgressRouteHandler(egress interfaces.IEgress) SyscallHandler {
	return func(did interfaces.DID, payload []byte) ([]byte, uint16) {
		if egress == nil {
			return nil, ErrInternalDaemon
		}

		data, _, status := ReadBytesParam(payload)
		if status != Success {
			return nil, status
		}

		err := egress.RouteToNetwork(did, data)
		if err != nil {
			return []byte(err.Error()), ErrInternalDaemon
		}

		return nil, Success
	}
}

// MakeGetInviteKeyHandler constructs a SyscallHandler for the OpcodeGetInviteKey system call.
// Parameter payload can specify a 4-byte uint32 BigEndian key version (defaults to 0 for v1 key type).
func MakeGetInviteKeyHandler(km *invitekey.KeyManager) SyscallHandler {
	return func(_ interfaces.DID, payload []byte) ([]byte, uint16) {
		if km == nil {
			return nil, ErrInternalDaemon
		}

		var version uint32
		if len(payload) >= 4 {
			version = binary.BigEndian.Uint32(payload[:4])
		}

		keyStr, err := km.GenerateKey(version)
		if err != nil {
			return []byte(err.Error()), ErrInvalidKey
		}

		return []byte(keyStr), Success
	}
}
