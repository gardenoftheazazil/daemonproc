// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package controlcenter_test

import (
	"encoding/binary"
	"testing"

	"github.com/gardenoftheazazil/daemonproc/controlcenter"
	"github.com/gardenoftheazazil/daemonproc/interfaces"
)

func FuzzControlCenterDispatcher(f *testing.F) {
	dispatcher, _ := setupIntegrationEnvironment(f)

	// Seed corpus.
	f.Add([]byte{})
	f.Add([]byte{0x01})

	// Valid GetInviteKey Opcode.
	validKeyReq := make([]byte, 2)
	binary.BigEndian.PutUint16(validKeyReq, controlcenter.OpcodeGetInviteKey)
	f.Add(validKeyReq)

	// Valid GetInviteKey with version payload.
	validKeyVerReq := make([]byte, 6)
	binary.BigEndian.PutUint16(validKeyVerReq[0:2], controlcenter.OpcodeGetInviteKey)
	binary.BigEndian.PutUint32(validKeyVerReq[2:6], 0)
	f.Add(validKeyVerReq)

	// Unknown Opcode.
	unknownReq := make([]byte, 10)
	binary.BigEndian.PutUint16(unknownReq, 0x7777)
	f.Add(unknownReq)

	f.Fuzz(func(t *testing.T, payload []byte) {
		did := interfaces.DID(12345)

		// DispatchSysCall must never panic regardless of payload content or length.
		res := dispatcher.DispatchSysCall(did, payload)

		if res.DID != did {
			t.Fatalf("expected DID %d in response, got: %d", did, res.DID)
		}

		// Verify encoding does not panic.
		encoded := res.Encode()
		if len(encoded) < 4 {
			t.Fatalf("encoded response must be at least 4 bytes, got: %d", len(encoded))
		}
	})
}
