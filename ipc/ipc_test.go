// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package ipc

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"sync"
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

func writeClientFrame(w io.Writer, isControl bool, data []byte) error {
	length := len(data)
	if length > maxPacketSize {
		return fmt.Errorf("packet length %d exceeds maximum of %d", length, maxPacketSize)
	}
	header := uint16(length) & 0x7FFF
	if isControl {
		header |= 0x8000
	}
	if err := binary.Write(w, binary.BigEndian, header); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func readClientFrame(r io.Reader) (isControl bool, payload []byte, err error) {
	var header uint16
	readErr := binary.Read(r, binary.BigEndian, &header)
	if readErr != nil {
		return false, nil, readErr
	}
	isControl = (header & 0x8000) != 0
	length := int(header & 0x7FFF)
	buf := make([]byte, length)
	_, readErr = io.ReadFull(r, buf)
	if readErr != nil {
		return false, nil, readErr
	}
	return isControl, buf, nil
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

	// Test SendToLocal (Data packet).
	testPayload := []byte("hello-local")
	go func() {
		_ = sm.SendToLocal(did, testPayload)
	}()

	// Read from client and verify framing.
	isCtrl, readBuf, readErr := readClientFrame(clientConn)
	if readErr != nil {
		t.Fatalf("failed to read client frame: %v", readErr)
	}
	if isCtrl {
		t.Fatal("expected data frame, got control frame")
	}
	if string(readBuf) != string(testPayload) {
		t.Fatalf("expected payload %s, got %s", testPayload, readBuf)
	}

	// Test RouteToNetwork (Data packet).
	networkPayload := []byte("hello-network")
	// Send to server from client using framing (data packet, isControl = false).
	if writeErr := writeClientFrame(clientConn, false, networkPayload); writeErr != nil {
		t.Fatalf("failed to write client frame: %v", writeErr)
	}

	// Wait for read loop to route the packet.
	time.Sleep(100 * time.Millisecond)

	megress.mu.Lock()
	sentData := megress.sentData
	sentDID := megress.sentDID
	megress.mu.Unlock()

	if string(sentData) != string(networkPayload) {
		t.Fatalf("expected routed data %s, got %s", networkPayload, sentData)
	}
	if sentDID != did {
		t.Fatalf("expected source DID %d, got %d", did, sentDID)
	}

	// Test Control Plane (Local Commands Callback).
	var receivedControlPayload []byte
	var controlWG sync.WaitGroup
	controlWG.Add(1)
	sm.SetControlCallback(func(cbDID interfaces.DID, payload []byte) {
		if cbDID == did {
			receivedControlPayload = payload
			// Write response back to local application.
			// Run in a goroutine to avoid blocking the read loop, preventing a deadlock on Windows named pipes.
			go func() {
				_ = sm.SendControlToLocal(cbDID, []byte("control-response"))
			}()
			controlWG.Done()
		}
	})

	controlRequestPayload := []byte("control-request")
	if writeErr := writeClientFrame(clientConn, true, controlRequestPayload); writeErr != nil {
		t.Fatalf("failed to write control request: %v", writeErr)
	}

	controlWG.Wait()
	if string(receivedControlPayload) != string(controlRequestPayload) {
		t.Fatalf("expected callback payload %s, got %s", controlRequestPayload, receivedControlPayload)
	}

	// Client reads response from control callback.
	isCtrl, respBuf, readErr := readClientFrame(clientConn)
	if readErr != nil {
		t.Fatalf("failed to read control response: %v", readErr)
	}
	if !isCtrl {
		t.Fatal("expected control frame response")
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
