// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package controlcenter

import (
	"encoding/binary"
	"errors"
	"sync"

	"github.com/gardenoftheazazil/daemonproc/interfaces"
)

// Status codes returned by the ABI control center dispatcher.
const (
	// Success indicates that the system call executed successfully.
	Success uint16 = 0x0000

	// ErrInvalidArgs indicates invalid parameter body or payload length mismatch.
	ErrInvalidArgs uint16 = 0x1001

	// ErrAuthFailed indicates authentication/DID verification failure.
	ErrAuthFailed uint16 = 0x1002

	// ErrUnknownOpcode indicates that the requested Opcode is not registered.
	ErrUnknownOpcode uint16 = 0x1003

	// ErrTooManyRequests indicates request rate limit exceeded for the session.
	ErrTooManyRequests uint16 = 0x1004

	// ErrInvalidKey indicates that the provided invite key is invalid or undecodable.
	ErrInvalidKey uint16 = 0x2001

	// ErrPeerTimeout indicates remote peer connection attempt timed out.
	ErrPeerTimeout uint16 = 0x2002

	// ErrInternalDaemon indicates an unhandled internal daemon error during syscall execution.
	ErrInternalDaemon uint16 = 0x5000
)

// ErrAlreadyRegistered is returned when attempting to register an Opcode that is already present in the table.
var ErrAlreadyRegistered = errors.New("syscall opcode already registered")

// IpcResponse represents the binary execution result returned by the control center dispatcher.
type IpcResponse struct {
	// StatusCode represents the execution result status.
	StatusCode uint16

	// Opcode represents the command identifier matching the originating request.
	Opcode uint16

	// Payload represents the binary execution result payload.
	Payload []byte

	// DID represents the target dispatch process identifier.
	DID interfaces.DID
}

// Encode serializes the IpcResponse into wire format bytes: [Opcode: 2B][StatusCode: 2B][Payload: VarB].
func (r IpcResponse) Encode() []byte {
	buf := make([]byte, 4+len(r.Payload))
	binary.BigEndian.PutUint16(buf[0:2], r.Opcode)
	binary.BigEndian.PutUint16(buf[2:4], r.StatusCode)
	copy(buf[4:], r.Payload)
	return buf
}

// SyscallHandler defines the internal function signature for parsing and processing binary ABI payloads.
type SyscallHandler func(did interfaces.DID, payload []byte) (response []byte, statusCode uint16)

// SyscallDescriptor represents a single registered system call transition layout within the ABI table.
type SyscallDescriptor struct {
	// Opcode specifies the unique 16-bit operation code.
	Opcode uint16

	// Name specifies the human-readable identifier of the system call.
	Name string

	// Handler specifies the execution function for the system call.
	Handler SyscallHandler
}

// Dispatcher operates as the high-mind engine that decodes raw IPC byte streams into targeted function calls.
type Dispatcher struct {
	syscalls map[uint16]SyscallDescriptor
	mutex    sync.RWMutex
}

// NewDispatcher initializes and returns a new Dispatcher instance.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		syscalls: make(map[uint16]SyscallDescriptor),
	}
}

// RegisterSysCall registers a SyscallDescriptor for a given 16-bit Opcode.
// It returns ErrAlreadyRegistered if the Opcode has already been registered.
func (d *Dispatcher) RegisterSysCall(opcode uint16, descriptor SyscallDescriptor) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.syscalls == nil {
		d.syscalls = make(map[uint16]SyscallDescriptor)
	}

	if _, exists := d.syscalls[opcode]; exists {
		return ErrAlreadyRegistered
	}

	d.syscalls[opcode] = descriptor
	return nil
}

// DispatchSysCall identifies the target system call by Opcode,
// executes its handler, and returns an IpcResponse structure.
func (d *Dispatcher) DispatchSysCall(did interfaces.DID, opcode uint16, payload []byte) IpcResponse {
	d.mutex.RLock()
	descriptor, exists := d.syscalls[opcode]
	d.mutex.RUnlock()

	if !exists {
		return IpcResponse{
			StatusCode: ErrUnknownOpcode,
			Opcode:     opcode,
			Payload:    nil,
			DID:        did,
		}
	}

	resPayload, statusCode := descriptor.Handler(did, payload)
	return IpcResponse{
		StatusCode: statusCode,
		Opcode:     opcode,
		Payload:    resPayload,
		DID:        did,
	}
}
