// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package controlcenter_test

import (
	"context"
	"crypto/sha512"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gardenoftheazazil/daemonproc/controlcenter"
	"github.com/gardenoftheazazil/daemonproc/interfaces"
	"github.com/gardenoftheazazil/daemonproc/ipc"
)

const (
	OpcodeHeavyCrypto   uint16 = 0x0301
	OpcodeBlockingIO    uint16 = 0x0302
	OpcodeMemoryAlloc   uint16 = 0x0303
	OpcodeStateMutation uint16 = 0x0304
)

type sharedStateCounter struct {
	mu    sync.Mutex
	count uint64
}

func registerHeavyTestSyscalls(tb testing.TB, d *controlcenter.Dispatcher) *sharedStateCounter {
	tb.Helper()

	counter := &sharedStateCounter{}

	// 1. CPU-bound Crypto Syscall (SHA-512 iteration loop).
	errCrypto := d.RegisterSysCall(OpcodeHeavyCrypto, controlcenter.SyscallDescriptor{
		Opcode: OpcodeHeavyCrypto,
		Name:   "HeavyCrypto",
		Handler: func(_ interfaces.DID, payload []byte) ([]byte, uint16) {
			hash := sha512.Sum512(payload)
			for range 200 {
				hash = sha512.Sum512(hash[:])
			}
			return hash[:32], controlcenter.Success
		},
	})
	if errCrypto != nil {
		tb.Fatalf("failed to register HeavyCrypto: %v", errCrypto)
	}

	// 2. Simulated Async I/O / P2P Handshake Delay Syscall.
	errIO := d.RegisterSysCall(OpcodeBlockingIO, controlcenter.SyscallDescriptor{
		Opcode: OpcodeBlockingIO,
		Name:   "BlockingIO",
		Handler: func(_ interfaces.DID, payload []byte) ([]byte, uint16) {
			time.Sleep(2 * time.Millisecond)
			return payload, controlcenter.Success
		},
	})
	if errIO != nil {
		tb.Fatalf("failed to register BlockingIO: %v", errIO)
	}

	// 3. Memory-Intensive Allocation Syscall (GC pressure).
	errMem := d.RegisterSysCall(OpcodeMemoryAlloc, controlcenter.SyscallDescriptor{
		Opcode: OpcodeMemoryAlloc,
		Name:   "MemoryAlloc",
		Handler: func(_ interfaces.DID, payload []byte) ([]byte, uint16) {
			buf := make([]byte, 512*1024) // 512KB allocation.
			for i := range buf {
				buf[i] = byte(i % 256)
			}
			res := sha512.Sum512(buf)
			return res[:16], controlcenter.Success
		},
	})
	if errMem != nil {
		tb.Fatalf("failed to register MemoryAlloc: %v", errMem)
	}

	// 4. Mutex Lock Contention State Mutation Syscall.
	errState := d.RegisterSysCall(OpcodeStateMutation, controlcenter.SyscallDescriptor{
		Opcode: OpcodeStateMutation,
		Name:   "StateMutation",
		Handler: func(_ interfaces.DID, _ []byte) ([]byte, uint16) {
			counter.mu.Lock()
			counter.count++
			val := counter.count
			counter.mu.Unlock()

			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, val)
			return buf, controlcenter.Success
		},
	})
	if errState != nil {
		tb.Fatalf("failed to register StateMutation: %v", errState)
	}

	return counter
}

func TestIPC_ControlCenter_HeavyWorkload_Stress(t *testing.T) {
	t.Parallel()

	dispatcher, _ := setupIntegrationEnvironment(t)
	counter := registerHeavyTestSyscalls(t, dispatcher)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sessionManager := ipc.NewSessionManager(ctx, &mockEgress{})
	defer func() {
		_ = sessionManager.Close()
	}()

	sessionManager.SetControlCallback(func(did interfaces.DID, payload []byte) {
		res := dispatcher.DispatchSysCall(did, payload)
		_ = sessionManager.SendControlToLocal(did, res.Encode())
	})

	numConcurrentApps := 30
	requestsPerApp := 20
	var wg sync.WaitGroup

	for appIdx := range numConcurrentApps {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			serverConn, clientConn := net.Pipe()
			defer serverConn.Close()
			defer clientConn.Close()

			did, errReg := sessionManager.RegisterSession(serverConn)
			if errReg != nil {
				t.Errorf("app %d: failed to register session: %v", id, errReg)
				return
			}

			opcodes := []uint16{
				controlcenter.OpcodeGetInviteKey,
				OpcodeHeavyCrypto,
				OpcodeBlockingIO,
				OpcodeMemoryAlloc,
				OpcodeStateMutation,
			}

			for reqIdx := range requestsPerApp {
				targetOpcode := opcodes[reqIdx%len(opcodes)]

				reqPayload := make([]byte, 6)
				binary.BigEndian.PutUint16(reqPayload[0:2], targetOpcode)
				binary.BigEndian.PutUint32(reqPayload[2:6], 0) // version 0 for v1 invite key.

				_ = clientConn.SetDeadline(time.Now().Add(10 * time.Second))
				if errWrite := writeIPCControlPacket(clientConn, reqPayload); errWrite != nil {
					t.Errorf("app %d (did %d) req %d: write failed: %v", id, did, reqIdx, errWrite)
					return
				}

				resOp, status, _, errRead := readIPCControlPacket(clientConn)
				if errRead != nil {
					t.Errorf("app %d (did %d) req %d: read failed: %v", id, did, reqIdx, errRead)
					return
				}

				if resOp != targetOpcode {
					t.Errorf("app %d req %d: expected opcode 0x%04X, got 0x%04X", id, reqIdx, targetOpcode, resOp)
				}
				if status != controlcenter.Success {
					t.Errorf("app %d req %d: expected status Success, got %d", id, reqIdx, status)
				}
			}
		}(appIdx)
	}

	wg.Wait()

	// Verify state mutation calls succeeded across all client apps.
	expectedMutations := uint64(numConcurrentApps * (requestsPerApp / 5))
	counter.mu.Lock()
	actualMutations := counter.count
	counter.mu.Unlock()

	if actualMutations != expectedMutations {
		t.Errorf("expected %d state mutations, got: %d", expectedMutations, actualMutations)
	}
}
