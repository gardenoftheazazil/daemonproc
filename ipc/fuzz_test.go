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
	// Seed 1: A valid data packet (isControl = false, payload = "hello").
	// Header: 5 bytes length -> 0x0005.
	f.Add([]byte{0x00, 0x05, 'h', 'e', 'l', 'l', 'o'})

	// Seed 2: A valid control packet (isControl = true, payload = `{"action":"test"}`).
	// Length: 17 -> 0x0011. MSB set -> 0x8011.
	f.Add([]byte{0x80, 0x11, '{', '"', 'a', 'c', 't', 'i', 'o', 'n', '"', ':', '"', 't', 'e', 's', 't', '"', '}'})

	// Seed 3: Incomplete headers or headers exceeding maxPacketSize.
	f.Add([]byte{0x00})
	f.Add([]byte{0x80, 0xFF}) // 32767 length, but missing payload.

	f.Fuzz(func(t *testing.T, data []byte) {
		sm := NewSessionManager(context.Background(), fuzzEgress{})
		defer func() {
			_ = sm.Close()
		}()

		r := bytes.NewReader(data)
		isControl, payload, err := sm.readPacket(r)
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
		if writeErr := sm.writePacket(&buf, isControl, payload); writeErr != nil {
			t.Errorf("failed to serialize parsed packet back: %v", writeErr)
		}
	})
}
