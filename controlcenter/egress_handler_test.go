// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package controlcenter_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/gardenoftheazazil/daemonproc/controlcenter"
	"github.com/gardenoftheazazil/daemonproc/interfaces"
)

type recordingEgress struct {
	mu          sync.Mutex
	lastDID     interfaces.DID
	lastData    []byte
	shouldError bool
}

func (r *recordingEgress) RouteToNetwork(srcDID interfaces.DID, data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.shouldError {
		return errors.New("network egress link failure")
	}
	r.lastDID = srcDID
	r.lastData = append([]byte(nil), data...)
	return nil
}

func TestMakeEgressRouteHandler_NilEgress(t *testing.T) {
	t.Parallel()

	handler := controlcenter.MakeEgressRouteHandler(nil)
	_, status := handler(1, []byte{0x00, 0x00})
	if status != controlcenter.ErrInternalDaemon {
		t.Fatalf("expected ErrInternalDaemon for nil egress, got: %d", status)
	}
}

func TestMakeEgressRouteHandler_EdgeCases(t *testing.T) {
	t.Parallel()

	egress := &recordingEgress{}
	handler := controlcenter.MakeEgressRouteHandler(egress)
	targetDID := interfaces.DID(100)

	tests := []struct {
		name       string
		payload    []byte
		wantStatus uint16
		wantData   []byte
	}{
		{
			name:       "nil payload",
			payload:    nil,
			wantStatus: controlcenter.ErrInvalidArgs,
			wantData:   nil,
		},
		{
			name:       "empty payload []byte{}",
			payload:    []byte{},
			wantStatus: controlcenter.ErrInvalidArgs,
			wantData:   nil,
		},
		{
			name:       "1 byte payload (missing 2-byte length header)",
			payload:    []byte{0x00},
			wantStatus: controlcenter.ErrInvalidArgs,
			wantData:   nil,
		},
		{
			name:       "length mismatch - declared 10 bytes, only 3 provided",
			payload:    []byte{0x00, 0x0A, 'a', 'b', 'c'},
			wantStatus: controlcenter.ErrInvalidArgs,
			wantData:   nil,
		},
		{
			name:       "valid empty packet (0 bytes declared)",
			payload:    []byte{0x00, 0x00},
			wantStatus: controlcenter.Success,
			wantData:   []byte{},
		},
		{
			name:       "valid network data packet (5 bytes declared and provided)",
			payload:    append([]byte{0x00, 0x05}, []byte("hello")...),
			wantStatus: controlcenter.Success,
			wantData:   []byte("hello"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			respPayload, status := handler(targetDID, tt.payload)

			if status != tt.wantStatus {
				t.Errorf("handler status = %d, wantStatus %d", status, tt.wantStatus)
			}

			if status == controlcenter.Success {
				if respPayload != nil {
					t.Errorf("expected nil response payload on success, got: %v", respPayload)
				}
				egress.mu.Lock()
				routedData := egress.lastData
				routedDID := egress.lastDID
				egress.mu.Unlock()

				if routedDID != targetDID {
					t.Errorf("routed DID = %d, want %d", routedDID, targetDID)
				}
				if !bytes.Equal(routedData, tt.wantData) {
					t.Errorf("routed data = %q, want %q", routedData, tt.wantData)
				}
			}
		})
	}
}

func TestMakeEgressRouteHandler_EgressFailure(t *testing.T) {
	t.Parallel()

	egress := &recordingEgress{shouldError: true}
	handler := controlcenter.MakeEgressRouteHandler(egress)

	validPayload := []byte{0x00, 0x04, 't', 'e', 's', 't'}
	errResp, status := handler(1, validPayload)

	if status != controlcenter.ErrInternalDaemon {
		t.Fatalf("expected ErrInternalDaemon on egress failure, got: %d", status)
	}
	if string(errResp) != "network egress link failure" {
		t.Fatalf("expected error string in response payload, got: %s", string(errResp))
	}
}

func TestMakeEgressRouteHandler_RaceCondition_Stress(t *testing.T) {
	t.Parallel()

	egress := &recordingEgress{}
	handler := controlcenter.MakeEgressRouteHandler(egress)

	workers := 50
	iterations := 200
	var wg sync.WaitGroup

	for w := range workers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			did := interfaces.DID(workerID + 1)
			dataStr := fmt.Sprintf("data-from-worker-%d", workerID)
			payload := make([]byte, 2+len(dataStr))
			binary.BigEndian.PutUint16(payload[0:2], uint16(len(dataStr)))
			copy(payload[2:], dataStr)

			for i := range iterations {
				// Alternate valid payloads and edge case invalid payloads to stress state machine.
				if i%2 == 0 {
					_, status := handler(did, payload)
					if status != controlcenter.Success {
						t.Errorf("worker %d iter %d: unexpected status %d", workerID, i, status)
					}
				} else {
					// Invalid short payload.
					_, status := handler(did, []byte{0x00, 0xFF})
					if status != controlcenter.ErrInvalidArgs {
						t.Errorf("worker %d iter %d: expected ErrInvalidArgs, got %d", workerID, i, status)
					}
				}
			}
		}(w)
	}

	wg.Wait()
}
