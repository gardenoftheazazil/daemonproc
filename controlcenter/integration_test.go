// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package controlcenter_test

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gardenoftheazazil/daemonproc/controlcenter"
	"github.com/gardenoftheazazil/daemonproc/interfaces"
	"github.com/gardenoftheazazil/daemonproc/invitekey"
	"github.com/gardenoftheazazil/daemonproc/ipc"
)

// mockEgress implements interfaces.IEgress for testing purposes.
type mockEgress struct{}

func (m *mockEgress) RouteToNetwork(srcDID interfaces.DID, data []byte) error {
	return nil
}

func setupIntegrationEnvironment(tb testing.TB) (*controlcenter.Dispatcher, *invitekey.KeyManager) {
	tb.Helper()

	km := invitekey.NewKeyManager()

	privKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		tb.Fatalf("failed to generate ecdh key: %v", err)
	}
	_, edPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		tb.Fatalf("failed to generate ed25519 key: %v", err)
	}

	genV1 := invitekey.NewKeyGeneratorV1(privKey.PublicKey(), "127.0.0.1:9000", edPriv)
	km.RegisterGenerator(0, genV1)

	dispatcher := controlcenter.NewDispatcher()
	if errReg := controlcenter.RegisterDefaultHandlers(dispatcher, km); errReg != nil {
		tb.Fatalf("failed to register default handlers: %v", errReg)
	}

	return dispatcher, km
}

func writeIPCControlPacket(w io.Writer, payload []byte) error {
	length := len(payload)
	header := uint16(length) | 0x8000
	if err := binary.Write(w, binary.BigEndian, header); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func readIPCControlPacket(r io.Reader) (opcode uint16, statusCode uint16, payload []byte, err error) {
	var header uint16
	if err := binary.Read(r, binary.BigEndian, &header); err != nil {
		return 0, 0, nil, err
	}

	length := int(header & 0x7FFF)
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, 0, nil, err
	}

	if len(buf) < 4 {
		return 0, 0, nil, io.ErrUnexpectedEOF
	}

	opcode = binary.BigEndian.Uint16(buf[0:2])
	statusCode = binary.BigEndian.Uint16(buf[2:4])
	payload = buf[4:]
	return opcode, statusCode, payload, nil
}

func TestIPC_ControlCenter_MultiAppIntegration(t *testing.T) {
	t.Parallel()

	dispatcher, _ := setupIntegrationEnvironment(t)

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

	numApps := 5
	var wg sync.WaitGroup

	for appIdx := range numApps {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			serverConn, clientConn := net.Pipe()
			defer serverConn.Close()
			defer clientConn.Close()

			did, err := sessionManager.RegisterSession(serverConn)
			if err != nil {
				t.Errorf("app %d: failed to register session: %v", id, err)
				return
			}
			if did == 0 {
				t.Errorf("app %d: expected non-zero DID", id)
				return
			}

			// 1. Send valid GetInviteKey syscall request over IPC channel.
			reqBody := make([]byte, 2)
			binary.BigEndian.PutUint16(reqBody, controlcenter.OpcodeGetInviteKey)

			_ = clientConn.SetDeadline(time.Now().Add(5 * time.Second))
			if errWrite := writeIPCControlPacket(clientConn, reqBody); errWrite != nil {
				t.Errorf("app %d: failed to write control packet: %v", id, errWrite)
				return
			}

			op, status, resPayload, errRead := readIPCControlPacket(clientConn)
			if errRead != nil {
				t.Errorf("app %d: failed to read response packet: %v", id, errRead)
				return
			}

			if op != controlcenter.OpcodeGetInviteKey {
				t.Errorf("app %d: expected Opcode 0x0201, got: 0x%04X", id, op)
			}
			if status != controlcenter.Success {
				t.Errorf("app %d: expected status Success, got: %d", id, status)
			}
			if !bytes.HasPrefix(resPayload, []byte("gota1-")) {
				t.Errorf("app %d: expected key starting with 'gota1-', got: %s", id, string(resPayload))
			}

			// 2. Send unknown Opcode request.
			unknownReq := make([]byte, 2)
			binary.BigEndian.PutUint16(unknownReq, 0x8888)

			if errWrite := writeIPCControlPacket(clientConn, unknownReq); errWrite != nil {
				t.Errorf("app %d: failed to write unknown opcode packet: %v", id, errWrite)
				return
			}

			opUnk, statusUnk, _, errReadUnk := readIPCControlPacket(clientConn)
			if errReadUnk != nil {
				t.Errorf("app %d: failed to read unknown response: %v", id, errReadUnk)
				return
			}

			if opUnk != 0x8888 {
				t.Errorf("app %d: expected Opcode 0x8888, got: 0x%04X", id, opUnk)
			}
			if statusUnk != controlcenter.ErrUnknownOpcode {
				t.Errorf("app %d: expected status ErrUnknownOpcode, got: %d", id, statusUnk)
			}
		}(appIdx)
	}

	wg.Wait()
}
