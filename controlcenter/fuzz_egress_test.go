// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package controlcenter_test

import (
	"testing"

	"github.com/gardenoftheazazil/daemonproc/controlcenter"
	"github.com/gardenoftheazazil/daemonproc/interfaces"
)

type nopEgress struct{}

func (nopEgress) RouteToNetwork(_ interfaces.DID, _ []byte) error {
	return nil
}

// FuzzEgressRouteHandler fuzzes the EgressRoute syscall handler with random binary payloads
// to ensure zero panics, out-of-bounds reads, or memory corruption occur under malformed inputs.
func FuzzEgressRouteHandler(f *testing.F) {
	// Seed 1: Empty payload.
	f.Add([]byte{})

	// Seed 2: 1 byte payload.
	f.Add([]byte{0x00})

	// Seed 3: Valid 0-length data packet header.
	f.Add([]byte{0x00, 0x00})

	// Seed 4: Valid 5-length data packet header.
	f.Add([]byte{0x00, 0x05, 'a', 'b', 'c', 'd', 'e'})

	// Seed 5: Length mismatch (declared 65535, 2 bytes provided).
	f.Add([]byte{0xFF, 0xFF})

	// Seed 6: Large length declared with truncated body.
	f.Add([]byte{0x01, 0x00, 'x', 'y', 'z'})

	handler := controlcenter.MakeEgressRouteHandler(nopEgress{})

	f.Fuzz(func(t *testing.T, payload []byte) {
		did := interfaces.DID(9999)

		// Executing handler must never panic under any arbitrary payload input.
		resPayload, statusCode := handler(did, payload)

		if statusCode == controlcenter.Success {
			if resPayload != nil {
				t.Fatalf("expected nil response payload on Success, got: %v", resPayload)
			}
		}
	})
}
