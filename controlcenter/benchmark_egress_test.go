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

func BenchmarkMakeEgressRouteHandler(b *testing.B) {
	handler := controlcenter.MakeEgressRouteHandler(nopEgress{})
	did := interfaces.DID(1)

	// Construct 1024-byte payload with 2-byte ABI length prefix.
	data := make([]byte, 1024)
	payload := make([]byte, 2+len(data))
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(data)))

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, status := handler(did, payload)
			if status != controlcenter.Success {
				b.Fatalf("unexpected status: %d", status)
			}
		}
	})
}
