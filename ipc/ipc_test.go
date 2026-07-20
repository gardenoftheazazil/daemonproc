// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package ipc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gardenoftheazazil/daemonproc/interfaces"
)

type mockEgress struct {
	mu       sync.Mutex
	sentDID  interfaces.DID
	sentData []byte
}

func (m *mockEgress) RouteToNetwork(srcDID interfaces.DID, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentDID = srcDID
	m.sentData = data
	return nil
}

func getTestSocketPath(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`\\.\pipe\test-gota-pipe-%d`, time.Now().UnixNano())
	}
	f, err := os.CreateTemp("", "test-gota-socket-*.sock")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	path := f.Name()
	_ = f.Close()
	_ = os.Remove(path)
	return path
}

func writeClientFrame(w io.Writer, opcode uint16, data []byte) error {
	length := len(data)
	if length > maxPacketSize {
		return fmt.Errorf("packet length %d exceeds maximum of %d", length, maxPacketSize)
	}
	var header [4]byte
	binary.BigEndian.PutUint16(header[0:2], opcode)
	binary.BigEndian.PutUint16(header[2:4], uint16(length))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	if length > 0 {
		_, err := w.Write(data)
		return err
	}
	return nil
}

func readClientFrame(r io.Reader) (opcode uint16, statusCode uint16, payload []byte, err error) {
	var header [6]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, 0, nil, err
	}
	opcode = binary.BigEndian.Uint16(header[0:2])
	statusCode = binary.BigEndian.Uint16(header[2:4])
	length := binary.BigEndian.Uint16(header[4:6])
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, 0, nil, err
	}
	return opcode, statusCode, buf, nil
}

func TestIPCListenerLifecycle(t *testing.T) {
	addr := getTestSocketPath(t)
	if runtime.GOOS != "windows" {
		defer func() {
			_ = os.Remove(addr)
		}()
	}

	listener := NewIpcListener(addr)
	ctx := t.Context()

	if err := listener.Start(ctx); err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}

	// Connect client.
	clientConnChan := make(chan net.Conn, 1)
	errChan := make(chan error, 1)
	go func() {
		conn, err := dialTestAddress(addr)
		if err != nil {
			errChan <- err
			return
		}
		clientConnChan <- conn
	}()

	// Accept client.
	serverConn, err := listener.Accept()
	if err != nil {
		t.Fatalf("failed to accept connection: %v", err)
	}
	defer func() {
		_ = serverConn.Close()
	}()

	select {
	case clientConn := <-clientConnChan:
		defer func() {
			_ = clientConn.Close()
		}()
	case err := <-errChan:
		t.Fatalf("failed to dial: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for client connection")
	}

	// Verify Stop cleans up resources.
	if err := listener.Stop(); err != nil {
		t.Fatalf("failed to stop listener: %v", err)
	}

	// Check that socket file is removed on Unix.
	if runtime.GOOS != "windows" {
		if _, err := os.Stat(addr); !os.IsNotExist(err) {
			t.Error("socket file was not removed after stop")
		}
	}
}

func TestSessionManager(t *testing.T) {
	addr := getTestSocketPath(t)
	if runtime.GOOS != "windows" {
		defer func() {
			_ = os.Remove(addr)
		}()
	}

	listener := NewIpcListener(addr)
	ctx := t.Context()

	if err := listener.Start(ctx); err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer func() {
		_ = listener.Stop()
	}()

	megress := &mockEgress{}
	sm := NewSessionManager(ctx, megress)
	defer func() {
		_ = sm.Close()
	}()

	// Connect client.
	var clientConn net.Conn
	var dialErr error
	var wg sync.WaitGroup
	wg.Go(func() {
		clientConn, dialErr = dialTestAddress(addr)
	})

	serverConn, err := listener.Accept()
	if err != nil {
		t.Fatalf("failed to accept connection: %v", err)
	}
	defer func() {
		_ = serverConn.Close()
	}()

	wg.Wait()
	if dialErr != nil {
		t.Fatalf("failed to dial: %v", dialErr)
	}
	defer func() {
		_ = clientConn.Close()
	}()

	// Register session.
	did, err := sm.RegisterSession(serverConn)
	if err != nil {
		t.Fatalf("failed to register session: %v", err)
	}
	if did == 0 {
		t.Fatal("expected non-zero DID")
	}

	// Test SendToLocal (Data packet with Opcode 0x0001, status 0x0000).
	testPayload := []byte("hello-local")
	go func() {
		_ = sm.SendToLocal(did, 0x0001, 0x0000, testPayload)
	}()

	// Read from client and verify framing.
	op, status, readBuf, readErr := readClientFrame(clientConn)
	if readErr != nil {
		t.Fatalf("failed to read client frame: %v", readErr)
	}
	if op != 0x0001 {
		t.Fatalf("expected opcode 0x0001, got 0x%04X", op)
	}
	if status != 0x0000 {
		t.Fatalf("expected status 0x0000, got 0x%04X", status)
	}
	if string(readBuf) != string(testPayload) {
		t.Fatalf("expected payload %s, got %s", testPayload, readBuf)
	}

	// Test Control Plane (Local Commands Callback).
	var receivedOpcode uint16
	var receivedControlPayload []byte
	var controlWG sync.WaitGroup
	controlWG.Add(1)
	sm.SetControlCallback(func(cbDID interfaces.DID, opcode uint16, payload []byte) {
		if cbDID == did {
			receivedOpcode = opcode
			receivedControlPayload = payload
			// Write response back to local application.
			go func() {
				_ = sm.SendToLocal(cbDID, opcode, 0x0000, []byte("control-response"))
			}()
			controlWG.Done()
		}
	})

	controlRequestPayload := []byte("control-request")
	if writeErr := writeClientFrame(clientConn, 0x0201, controlRequestPayload); writeErr != nil {
		t.Fatalf("failed to write control request: %v", writeErr)
	}

	controlWG.Wait()
	if receivedOpcode != 0x0201 {
		t.Fatalf("expected opcode 0x0201, got 0x%04X", receivedOpcode)
	}
	if string(receivedControlPayload) != string(controlRequestPayload) {
		t.Fatalf("expected callback payload %s, got %s", controlRequestPayload, receivedControlPayload)
	}

	// Client reads response from control callback.
	resOp, resStatus, respBuf, readErr := readClientFrame(clientConn)
	if readErr != nil {
		t.Fatalf("failed to read control response: %v", readErr)
	}
	if resOp != 0x0201 {
		t.Fatalf("expected response opcode 0x0201, got 0x%04X", resOp)
	}
	if resStatus != 0x0000 {
		t.Fatalf("expected response status 0x0000, got 0x%04X", resStatus)
	}
	if string(respBuf) != "control-response" {
		t.Fatalf("expected response payload 'control-response', got '%s'", respBuf)
	}

	// Test UnregisterSession.
	if unregErr := sm.UnregisterSession(did); unregErr != nil {
		t.Fatalf("failed to unregister session: %v", unregErr)
	}

	// Verify that client connection is closed.
	oneByte := make([]byte, 1)
	_ = clientConn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	_, readErr = clientConn.Read(oneByte)
	if readErr == nil {
		t.Fatal("expected error reading from closed connection, got nil")
	}
}

func TestSessionManager_ConcurrentSendToLocal(t *testing.T) {
	t.Parallel()

	addr := getTestSocketPath(t)
	if runtime.GOOS != "windows" {
		defer func() {
			_ = os.Remove(addr)
		}()
	}

	listener := NewIpcListener(addr)
	ctx := t.Context()

	if err := listener.Start(ctx); err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer func() {
		_ = listener.Stop()
	}()

	sm := NewSessionManager(ctx, &mockEgress{})
	defer func() {
		_ = sm.Close()
	}()

	var clientConn net.Conn
	var dialErr error
	var wg sync.WaitGroup
	wg.Go(func() {
		clientConn, dialErr = dialTestAddress(addr)
	})

	serverConn, err := listener.Accept()
	if err != nil {
		t.Fatalf("failed to accept connection: %v", err)
	}
	defer func() {
		_ = serverConn.Close()
	}()

	wg.Wait()
	if dialErr != nil {
		t.Fatalf("failed to dial: %v", dialErr)
	}
	defer func() {
		_ = clientConn.Close()
	}()

	did, err := sm.RegisterSession(serverConn)
	if err != nil {
		t.Fatalf("failed to register session: %v", err)
	}

	numWriters := 30
	msgsPerWriter := 20
	totalMsgs := numWriters * msgsPerWriter

	var sendWG sync.WaitGroup
	for w := range numWriters {
		sendWG.Add(1)
		go func(writerID int) {
			defer sendWG.Done()
			for i := range msgsPerWriter {
				payload := []byte(fmt.Sprintf("msg-writer-%d-idx-%d", writerID, i))
				_ = sm.SendToLocal(did, 0x0001, 0x0000, payload)
			}
		}(w)
	}

	// Read all messages on client side and verify no framing corruption occurs.
	readCount := 0
	for range totalMsgs {
		_ = clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		op, status, payload, readErr := readClientFrame(clientConn)
		if readErr != nil {
			t.Fatalf("read failed on msg %d: %v (corrupted framing or data race)", readCount, readErr)
		}
		if op != 0x0001 || status != 0x0000 {
			t.Fatalf("invalid header on msg %d: op=0x%04X status=0x%04X", readCount, op, status)
		}
		if len(payload) == 0 {
			t.Fatalf("empty payload on msg %d", readCount)
		}
		readCount++
	}

	sendWG.Wait()
	if readCount != totalMsgs {
		t.Fatalf("expected %d messages, got %d", totalMsgs, readCount)
	}
}

func TestSessionManager_ConcurrentAndDoubleUnregister(t *testing.T) {
	t.Parallel()

	addr := getTestSocketPath(t)
	if runtime.GOOS != "windows" {
		defer func() {
			_ = os.Remove(addr)
		}()
	}

	listener := NewIpcListener(addr)
	ctx := t.Context()

	if err := listener.Start(ctx); err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer func() {
		_ = listener.Stop()
	}()

	sm := NewSessionManager(ctx, &mockEgress{})
	defer func() {
		_ = sm.Close()
	}()

	var closeCallbackCount atomic.Uint32
	sm.SetOnClose(func(_ interfaces.DID) {
		closeCallbackCount.Add(1)
	})

	var clientConn net.Conn
	var dialErr error
	var wg sync.WaitGroup
	wg.Go(func() {
		clientConn, dialErr = dialTestAddress(addr)
	})

	serverConn, err := listener.Accept()
	if err != nil {
		t.Fatalf("failed to accept connection: %v", err)
	}
	defer func() {
		_ = serverConn.Close()
	}()

	wg.Wait()
	if dialErr != nil {
		t.Fatalf("failed to dial: %v", dialErr)
	}
	defer func() {
		_ = clientConn.Close()
	}()

	did, err := sm.RegisterSession(serverConn)
	if err != nil {
		t.Fatalf("failed to register session: %v", err)
	}

	// Concurrently call UnregisterSession from 20 goroutines and also close client connection.
	numGoroutines := 20
	var unregWG sync.WaitGroup
	for i := range numGoroutines {
		unregWG.Add(1)
		go func(id int) {
			defer unregWG.Done()
			if id%2 == 0 {
				_ = clientConn.Close()
			}
			_ = sm.UnregisterSession(did)
		}(i)
	}

	unregWG.Wait()

	// Wait briefly to allow read loop to exit cleanly.
	time.Sleep(50 * time.Millisecond)

	// Verify onClose callback was executed EXACTLY ONCE.
	if count := closeCallbackCount.Load(); count != 1 {
		t.Fatalf("expected onClose callback to be executed exactly 1 time, got: %d", count)
	}
}

func TestSessionManager_SlowClientProtection(t *testing.T) {
	t.Parallel()

	addr := getTestSocketPath(t)
	if runtime.GOOS != "windows" {
		defer func() {
			_ = os.Remove(addr)
		}()
	}

	listener := NewIpcListener(addr)
	ctx := t.Context()

	if err := listener.Start(ctx); err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer func() {
		_ = listener.Stop()
	}()

	sm := NewSessionManager(ctx, &mockEgress{})
	defer func() {
		_ = sm.Close()
	}()

	var clientConn net.Conn
	var dialErr error
	var wg sync.WaitGroup
	wg.Go(func() {
		clientConn, dialErr = dialTestAddress(addr)
	})

	serverConn, err := listener.Accept()
	if err != nil {
		t.Fatalf("failed to accept connection: %v", err)
	}
	defer func() {
		_ = serverConn.Close()
	}()

	wg.Wait()
	if dialErr != nil {
		t.Fatalf("failed to dial: %v", dialErr)
	}
	defer func() {
		_ = clientConn.Close()
	}()

	did, err := sm.RegisterSession(serverConn)
	if err != nil {
		t.Fatalf("failed to register session: %v", err)
	}

	// Client intentionally DOES NOT read from socket (simulating unresponsive/slow client).
	// We send messages until the outbound queue (1024 messages) is full.
	var blockedErr error
	start := time.Now()
	for range 2500 {
		errSend := sm.SendToLocal(did, 0x0001, 0x0000, []byte("overflow-data"))
		if errors.Is(errSend, ErrSessionBlocked) {
			blockedErr = errSend
			break
		}
	}
	elapsed := time.Since(start)

	if blockedErr == nil {
		t.Fatal("expected ErrSessionBlocked for slow client overflow")
	}

	// Ensure SendToLocal returned in sub-100ms without blocking for seconds.
	if elapsed > 500*time.Millisecond {
		t.Fatalf("SendToLocal blocked for %v, expected non-blocking immediate return", elapsed)
	}

	// Wait briefly for asynchronous unregistration.
	time.Sleep(100 * time.Millisecond)

	// Session should now be unregistered and cleaned up.
	errAfterDrop := sm.SendToLocal(did, 0x0001, 0x0000, []byte("post-drop"))
	if errAfterDrop == nil {
		t.Fatal("expected error sending to unregistered slow session")
	}
}
