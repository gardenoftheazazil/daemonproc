// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package ipc

import (
	"bytes"
	"context"
	"testing"

	"github.com/gardenoftheazazil/daemonproc/interfaces"
)

type fuzzEgress struct{}

func (fuzzEgress) RouteToNetwork(srcDID interfaces.DID, data []byte) error {
	return nil
}

// FuzzReadPacket fuzzes the IPC packet parser to ensure robustness against malformed
// or adversarial inputs, verifying there are no panics, buffer overflows, or memory leaks.
func FuzzReadPacket(f *testing.F) {
	// Add seed corpora.
	// Seed 1: A valid packet (Opcode = 0x0001, payload = "hello", len = 5).
	f.Add([]byte{0x00, 0x01, 0x00, 0x05, 'h', 'e', 'l', 'l', 'o'})

	// Seed 2: A valid control packet (Opcode = 0x0201, payload = `{"action":"test"}`, len = 17).
	f.Add([]byte{0x02, 0x01, 0x00, 0x11, '{', '"', 'a', 'c', 't', 'i', 'o', 'n', '"', ':', '"', 't', 'e', 's', 't', '"', '}'})

	// Seed 3: Incomplete headers.
	f.Add([]byte{0x00})
	f.Add([]byte{0x00, 0x01, 0xFF, 0xFF}) // Max length, but missing payload.

	f.Fuzz(func(t *testing.T, data []byte) {
		sm := NewSessionManager(context.Background(), fuzzEgress{})
		defer func() {
			_ = sm.Close()
		}()

		r := bytes.NewReader(data)
		opcode, payload, err := sm.readPacket(r)
		if err != nil {
			// Malformed packets are expected to return errors.
			return
		}

		// If parsing succeeded, verify constraints.
		if len(payload) > maxPacketSize {
			t.Errorf("parsed payload size %d exceeds maxPacketSize %d", len(payload), maxPacketSize)
		}

		// Ensure we can successfully write the parsed packet back.
		var buf bytes.Buffer
		if writeErr := sm.writePacket(&buf, opcode, 0x0000, payload); writeErr != nil {
			t.Errorf("failed to serialize parsed packet back: %v", writeErr)
		}
	})
}
