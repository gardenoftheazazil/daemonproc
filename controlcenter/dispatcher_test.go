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

func TestNewDispatcher(t *testing.T) {
	t.Parallel()

	d := controlcenter.NewDispatcher()
	if d == nil {
		t.Fatal("expected non-nil Dispatcher instance")
	}
}

func TestRegisterSysCall(t *testing.T) {
	t.Parallel()

	d := controlcenter.NewDispatcher()
	opcode := uint16(0x0100)

	desc := controlcenter.SyscallDescriptor{
		Opcode: opcode,
		Name:   "TestCall",
		Handler: func(did interfaces.DID, payload []byte) ([]byte, uint16) {
			return []byte("ok"), controlcenter.Success
		},
	}

	err := d.RegisterSysCall(opcode, desc)
	if err != nil {
		t.Fatalf("unexpected error registering syscall: %v", err)
	}

	// Duplicate registration should return ErrAlreadyRegistered.
	errDup := d.RegisterSysCall(opcode, desc)
	if !errors.Is(errDup, controlcenter.ErrAlreadyRegistered) {
		t.Fatalf("expected ErrAlreadyRegistered, got: %v", errDup)
	}
}

func TestDispatchSysCall_Success(t *testing.T) {
	t.Parallel()

	d := controlcenter.NewDispatcher()
	targetDID := interfaces.DID(42)
	opcode := uint16(0x0201)

	desc := controlcenter.SyscallDescriptor{
		Opcode: opcode,
		Name:   "GetInviteKey",
		Handler: func(did interfaces.DID, payload []byte) ([]byte, uint16) {
			if did != targetDID {
				return nil, controlcenter.ErrAuthFailed
			}
			response := append([]byte("KEY:"), payload...)
			return response, controlcenter.Success
		},
	}

	if err := d.RegisterSysCall(opcode, desc); err != nil {
		t.Fatalf("failed to register syscall: %v", err)
	}

	// Construct request payload: [2-byte Opcode][Parameter Bytes].
	paramData := []byte("v1_profile")
	reqPayload := make([]byte, 2+len(paramData))
	binary.BigEndian.PutUint16(reqPayload[0:2], opcode)
	copy(reqPayload[2:], paramData)

	res := d.DispatchSysCall(targetDID, reqPayload)

	if res.StatusCode != controlcenter.Success {
		t.Errorf("expected StatusCode Success (%d), got: %d", controlcenter.Success, res.StatusCode)
	}
	if res.Opcode != opcode {
		t.Errorf("expected Opcode 0x%04X, got: 0x%04X", opcode, res.Opcode)
	}
	if res.DID != targetDID {
		t.Errorf("expected DID %d, got: %d", targetDID, res.DID)
	}

	expectedPayload := []byte("KEY:v1_profile")
	if !bytes.Equal(res.Payload, expectedPayload) {
		t.Errorf("expected payload %q, got %q", expectedPayload, res.Payload)
	}
}

func TestDispatchSysCall_ShortPayload(t *testing.T) {
	t.Parallel()

	d := controlcenter.NewDispatcher()
	did := interfaces.DID(10)

	// Test 0-length payload.
	resEmpty := d.DispatchSysCall(did, []byte{})
	if resEmpty.StatusCode != controlcenter.ErrInvalidArgs {
		t.Errorf("expected ErrInvalidArgs for empty payload, got: %d", resEmpty.StatusCode)
	}

	// Test 1-byte payload.
	resShort := d.DispatchSysCall(did, []byte{0x01})
	if resShort.StatusCode != controlcenter.ErrInvalidArgs {
		t.Errorf("expected ErrInvalidArgs for 1-byte payload, got: %d", resShort.StatusCode)
	}
}

func TestDispatchSysCall_UnknownOpcode(t *testing.T) {
	t.Parallel()

	d := controlcenter.NewDispatcher()
	did := interfaces.DID(15)

	unknownOpcode := uint16(0x9999)
	reqPayload := make([]byte, 2)
	binary.BigEndian.PutUint16(reqPayload, unknownOpcode)

	res := d.DispatchSysCall(did, reqPayload)
	if res.StatusCode != controlcenter.ErrUnknownOpcode {
		t.Errorf("expected ErrUnknownOpcode, got: %d", res.StatusCode)
	}
	if res.Opcode != unknownOpcode {
		t.Errorf("expected Opcode 0x%04X, got: 0x%04X", unknownOpcode, res.Opcode)
	}
}

func TestIpcResponse_Encode(t *testing.T) {
	t.Parallel()

	res := controlcenter.IpcResponse{
		StatusCode: controlcenter.Success,
		Opcode:     0x0100,
		Payload:    []byte("hello"),
		DID:        100,
	}

	encoded := res.Encode()
	expectedLen := 4 + len("hello")
	if len(encoded) != expectedLen {
		t.Fatalf("expected encoded length %d, got %d", expectedLen, len(encoded))
	}

	opcode := binary.BigEndian.Uint16(encoded[0:2])
	if opcode != 0x0100 {
		t.Errorf("expected Opcode 0x0100, got: 0x%04X", opcode)
	}

	status := binary.BigEndian.Uint16(encoded[2:4])
	if status != controlcenter.Success {
		t.Errorf("expected status Success, got: %d", status)
	}

	if !bytes.Equal(encoded[4:], []byte("hello")) {
		t.Errorf("expected payload %q, got %q", "hello", encoded[4:])
	}
}

func TestDispatcher_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	d := controlcenter.NewDispatcher()
	numWorkers := 20
	var wg sync.WaitGroup

	// Register 10 syscalls.
	for i := range 10 {
		opcode := uint16(0x0100 + i)
		err := d.RegisterSysCall(opcode, controlcenter.SyscallDescriptor{
			Opcode: opcode,
			Name:   fmt.Sprintf("Call_%d", i),
			Handler: func(did interfaces.DID, payload []byte) ([]byte, uint16) {
				return payload, controlcenter.Success
			},
		})
		if err != nil {
			t.Fatalf("failed to register opcode 0x%04X: %v", opcode, err)
		}
	}

	// Concurrently dispatch calls.
	for worker := range numWorkers {
		wg.Add(1)
		go func(wID int) {
			defer wg.Done()
			for i := range 50 {
				op := uint16(0x0100 + (i % 10))
				buf := make([]byte, 4)
				binary.BigEndian.PutUint16(buf[0:2], op)
				binary.BigEndian.PutUint16(buf[2:4], uint16(wID))

				res := d.DispatchSysCall(interfaces.DID(wID), buf)
				if res.StatusCode != controlcenter.Success {
					t.Errorf("worker %d: unexpected status %d", wID, res.StatusCode)
				}
			}
		}(worker)
	}

	wg.Wait()
}
