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

	// Seed corpus: uint16 opcode, []byte payload.
	f.Add(controlcenter.OpcodeGetInviteKey, []byte{})

	// Valid GetInviteKey with version payload.
	verPayload := make([]byte, 4)
	binary.BigEndian.PutUint32(verPayload, 0)
	f.Add(controlcenter.OpcodeGetInviteKey, verPayload)

	// Unknown Opcode.
	f.Add(uint16(0x7777), []byte("arbitrary-data"))

	f.Fuzz(func(t *testing.T, opcode uint16, payload []byte) {
		did := interfaces.DID(12345)

		// DispatchSysCall must never panic regardless of payload content or length.
		res := dispatcher.DispatchSysCall(did, opcode, payload)

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
