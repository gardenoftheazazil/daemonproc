// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package controlcenter_test

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/gardenoftheazazil/daemonproc/controlcenter"
	"github.com/gardenoftheazazil/daemonproc/interfaces"
	"github.com/gardenoftheazazil/daemonproc/invitekey"
	"github.com/gardenoftheazazil/daemonproc/ipc"
)

func BenchmarkDispatcher_DispatchSysCall(b *testing.B) {
	dispatcher := controlcenter.NewDispatcher()
	opcode := uint16(0x0100)

	_ = dispatcher.RegisterSysCall(opcode, controlcenter.SyscallDescriptor{
		Opcode: opcode,
		Name:   "FastBenchmarkCall",
		Handler: func(_ interfaces.DID, payload []byte) ([]byte, uint16) {
			return payload, controlcenter.Success
		},
	})

	reqPayload := make([]byte, 34)
	binary.BigEndian.PutUint16(reqPayload[0:2], opcode)
	copy(reqPayload[2:], []byte("01234567890123456789012345678901"))

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		var didCounter interfaces.DID = 1
		for pb.Next() {
			res := dispatcher.DispatchSysCall(didCounter, reqPayload)
			if res.StatusCode != controlcenter.Success {
				b.Fatalf("unexpected status: %d", res.StatusCode)
			}
			didCounter++
		}
	})
}

func BenchmarkDispatcher_GetInviteKey(b *testing.B) {
	km := invitekey.NewKeyManager()
	privKey, _ := ecdh.X25519().GenerateKey(rand.Reader)
	_, edPriv, _ := ed25519.GenerateKey(rand.Reader)
	genV1 := invitekey.NewKeyGeneratorV1(privKey.PublicKey(), "127.0.0.1:9000", edPriv)
	km.RegisterGenerator(0, genV1)

	dispatcher := controlcenter.NewDispatcher()
	_ = dispatcher.RegisterSysCall(controlcenter.OpcodeGetInviteKey, controlcenter.SyscallDescriptor{
		Opcode:  controlcenter.OpcodeGetInviteKey,
		Name:    "GetInviteKey",
		Handler: controlcenter.MakeGetInviteKeyHandler(km),
	})

	reqPayload := make([]byte, 2)
	binary.BigEndian.PutUint16(reqPayload[0:2], controlcenter.OpcodeGetInviteKey)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		var didCounter interfaces.DID = 100
		for pb.Next() {
			res := dispatcher.DispatchSysCall(didCounter, reqPayload)
			if res.StatusCode != controlcenter.Success {
				b.Fatalf("unexpected status: %d", res.StatusCode)
			}
			didCounter++
		}
	})
}

func BenchmarkIPC_ControlCenter_EndToEnd(b *testing.B) {
	dispatcher := controlcenter.NewDispatcher()
	opcode := uint16(0x0100)
	_ = dispatcher.RegisterSysCall(opcode, controlcenter.SyscallDescriptor{
		Opcode: opcode,
		Name:   "Echo",
		Handler: func(_ interfaces.DID, payload []byte) ([]byte, uint16) {
			return payload, controlcenter.Success
		},
	})

	ctx := b.Context()

	sessionManager := ipc.NewSessionManager(ctx, &mockEgress{})
	defer func() {
		_ = sessionManager.Close()
	}()

	sessionManager.SetControlCallback(func(did interfaces.DID, payload []byte) {
		res := dispatcher.DispatchSysCall(did, payload)
		_ = sessionManager.SendControlToLocal(did, res.Encode())
	})

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	_, err := sessionManager.RegisterSession(serverConn)
	if err != nil {
		b.Fatalf("failed to register session: %v", err)
	}

	reqPayload := make([]byte, 10)
	binary.BigEndian.PutUint16(reqPayload[0:2], opcode)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = clientConn.SetDeadline(time.Now().Add(2 * time.Second))
		if errWrite := writeIPCControlPacket(clientConn, reqPayload); errWrite != nil {
			b.Fatalf("write failed: %v", errWrite)
		}
		_, status, _, errRead := readIPCControlPacket(clientConn)
		if errRead != nil {
			b.Fatalf("read failed: %v", errRead)
		}
		if status != controlcenter.Success {
			b.Fatalf("unexpected status: %d", status)
		}
	}
}

func BenchmarkIPC_ControlCenter_HeavyWorkload(b *testing.B) {
	dispatcher, _ := setupIntegrationEnvironment(b)
	_ = registerHeavyTestSyscalls(b, dispatcher)

	ctx := b.Context()

	sessionManager := ipc.NewSessionManager(ctx, &mockEgress{})
	defer func() {
		_ = sessionManager.Close()
	}()

	sessionManager.SetControlCallback(func(did interfaces.DID, payload []byte) {
		res := dispatcher.DispatchSysCall(did, payload)
		_ = sessionManager.SendControlToLocal(did, res.Encode())
	})

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	_, errReg := sessionManager.RegisterSession(serverConn)
	if errReg != nil {
		b.Fatalf("failed to register session: %v", errReg)
	}

	reqPayload := make([]byte, 2)
	binary.BigEndian.PutUint16(reqPayload[0:2], OpcodeHeavyCrypto)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = clientConn.SetDeadline(time.Now().Add(5 * time.Second))
		if errWrite := writeIPCControlPacket(clientConn, reqPayload); errWrite != nil {
			b.Fatalf("write failed: %v", errWrite)
		}
		_, status, _, errRead := readIPCControlPacket(clientConn)
		if errRead != nil {
			b.Fatalf("read failed: %v", errRead)
		}
		if status != controlcenter.Success {
			b.Fatalf("unexpected status: %d", status)
		}
	}
}
