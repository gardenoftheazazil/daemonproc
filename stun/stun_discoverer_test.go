// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package stun

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gardenoftheazazil/daemonproc/identify"
)

type mockUDPPortListener struct {
	mu           sync.Mutex
	sendCallback func(targetAddr *net.UDPAddr, data []byte) error
	subID        uint32
	callback     func(sourceAddr *net.UDPAddr, data []byte) bool
	packetChan   chan []byte
}

func (m *mockUDPPortListener) StartListener(ctx context.Context, port uint16) error {
	return nil
}

func (m *mockUDPPortListener) StopListener() error {
	return nil
}

func (m *mockUDPPortListener) SendPacketTo(targetAddr *net.UDPAddr, data []byte) error {
	m.mu.Lock()
	cb := m.sendCallback
	m.mu.Unlock()
	if cb != nil {
		return cb(targetAddr, data)
	}
	return nil
}

func (m *mockUDPPortListener) SubscribeToPacketReceived(
	callback func(sourceAddr *net.UDPAddr, data []byte) bool,
	packetChannel chan []byte,
) (uint32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subID++
	m.callback = callback
	m.packetChan = packetChannel
	return m.subID, nil
}

func (m *mockUDPPortListener) UnsubscribeFromPacketReceived(subscriptionID uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if subscriptionID == m.subID {
		m.callback = nil
		m.packetChan = nil
	}
	return nil
}

func (m *mockUDPPortListener) triggerReceive(sourceAddr *net.UDPAddr, data []byte) {
	m.mu.Lock()
	cb := m.callback
	ch := m.packetChan
	m.mu.Unlock()

	if cb != nil && cb(sourceAddr, data) {
		select {
		case ch <- data:
		default:
		}
	}
}

func TestParseSTUNResponseXORMappedIPv4(t *testing.T) {
	// Expected TxID.
	var txID [12]byte
	copy(txID[:], "123456789012")

	// Construct XOR-MAPPED-ADDRESS response for IPv4.
	// Target IP: 198.51.100.2, Target Port: 12345.
	// Port 12345 (0x3039) XOR 0x2112 = 0x112B.
	// IP 198.51.100.2 (0xC6336402) XOR 0x2112A442 = 0xE721C040.
	data := make([]byte, 32)
	binary.BigEndian.PutUint16(data[0:2], 0x0101)     // Success Response.
	binary.BigEndian.PutUint16(data[2:4], 8)          // Attribute Length = 8.
	binary.BigEndian.PutUint32(data[4:8], 0x2112A442) // Magic Cookie.
	copy(data[8:20], txID[:])

	// XOR-MAPPED-ADDRESS Attribute (Type 0x0020, Len 8).
	binary.BigEndian.PutUint16(data[20:22], 0x0020)
	binary.BigEndian.PutUint16(data[22:24], 8)
	data[24] = 0    // Reserved.
	data[25] = 0x01 // IPv4 Family.
	binary.BigEndian.PutUint16(data[26:28], 0x3039^0x2112)

	// Put XOR-Address value (4 bytes).
	binary.BigEndian.PutUint32(data[28:32], 0xC6336402^0x2112A442)

	// Update attribute length in message header to 12.
	binary.BigEndian.PutUint16(data[2:4], 12)

	addr, err := parseSTUNResponse(data, txID)
	if err != nil {
		t.Fatalf("unexpected error parsing stun response: %v", err)
	}

	if addr.IP.String() != "198.51.100.2" {
		t.Errorf("expected IP %s, got %s", "198.51.100.2", addr.IP.String())
	}
	if addr.Port != 12345 {
		t.Errorf("expected port %d, got %d", 12345, addr.Port)
	}
}

func TestParseSTUNResponseMappedIPv4(t *testing.T) {
	// Expected TxID.
	var txID [12]byte
	copy(txID[:], "123456789012")

	// Construct MAPPED-ADDRESS response for IPv4.
	// Target IP: 198.51.100.2, Target Port: 12345.
	data := make([]byte, 32)
	binary.BigEndian.PutUint16(data[0:2], 0x0101)     // Success Response.
	binary.BigEndian.PutUint16(data[2:4], 8)          // Attribute Length = 8.
	binary.BigEndian.PutUint32(data[4:8], 0x2112A442) // Magic Cookie.
	copy(data[8:20], txID[:])

	// MAPPED-ADDRESS Attribute (Type 0x0001, Len 8).
	binary.BigEndian.PutUint16(data[20:22], 0x0001)
	binary.BigEndian.PutUint16(data[22:24], 8)
	data[24] = 0    // Reserved.
	data[25] = 0x01 // IPv4 Family.
	binary.BigEndian.PutUint16(data[26:28], 12345)

	// Put Address value (4 bytes).
	copy(data[28:32], net.ParseIP("198.51.100.2").To4())

	// Update attribute length in message header to 12.
	binary.BigEndian.PutUint16(data[2:4], 12)

	addr, err := parseSTUNResponse(data, txID)
	if err != nil {
		t.Fatalf("unexpected error parsing stun response: %v", err)
	}

	if addr.IP.String() != "198.51.100.2" {
		t.Errorf("expected IP %s, got %s", "198.51.100.2", addr.IP.String())
	}
	if addr.Port != 12345 {
		t.Errorf("expected port %d, got %d", 12345, addr.Port)
	}
}

func TestDiscovererLoopSuccess(t *testing.T) {
	// Setup Identity.
	reader := rand.Reader
	xPriv, err := ecdh.X25519().GenerateKey(reader)
	if err != nil {
		t.Fatalf("failed to generate X25519 key: %v", err)
	}
	_, edPriv, err := ed25519.GenerateKey(reader)
	if err != nil {
		t.Fatalf("failed to generate Ed25519 key: %v", err)
	}
	id, err := identify.NewIdentity(xPriv, edPriv)
	if err != nil {
		t.Fatalf("failed to create Identity: %v", err)
	}

	// Setup Mock Port Listener.
	mockListener := &mockUDPPortListener{}

	// When discoverer sends a STUN request, reply with a mapped address.
	mockListener.sendCallback = func(targetAddr *net.UDPAddr, data []byte) error {
		if len(data) < 20 {
			return nil
		}
		var txID [12]byte
		copy(txID[:], data[8:20])

		// Reply XOR-MAPPED-ADDRESS.
		// Target: 203.0.113.50:54321.
		// Port: 54321 (0xD431) XOR 0xD431 = 0xF523.
		// IP: 203.0.113.50 (0xCB007132) XOR 0x2112A442 = 0xEA12D570.
		resp := make([]byte, 32)
		binary.BigEndian.PutUint16(resp[0:2], 0x0101)     // Success Response.
		binary.BigEndian.PutUint16(resp[2:4], 12)         // Attribute Length = 12.
		binary.BigEndian.PutUint32(resp[4:8], 0x2112A442) // Magic Cookie.
		copy(resp[8:20], txID[:])

		binary.BigEndian.PutUint16(resp[20:22], 0x0020) // Type: XOR-MAPPED-ADDRESS.
		binary.BigEndian.PutUint16(resp[22:24], 8)      // Length: 8.
		resp[24] = 0                                    // Reserved.
		resp[25] = 0x01                                 // IPv4.
		binary.BigEndian.PutUint16(resp[26:28], 54321^0x2112)
		binary.BigEndian.PutUint32(resp[28:32], 0xCB007132^0x2112A442)

		// Trigger receiving of this packet.
		go mockListener.triggerReceive(targetAddr, resp)
		return nil
	}

	// Setup Discoverer.
	disc := NewDiscoverer(mockListener, []string{"127.0.0.1:19302"}, id, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = disc.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start discoverer: %v", err)
	}
	defer func() {
		_ = disc.Stop()
	}()

	// Wait up to 1 second for the identity to be updated with the public IP.
	var finalAddr *net.UDPAddr
	for range 10 {
		finalAddr = id.GetPublicAddr()
		if finalAddr != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if finalAddr == nil {
		t.Fatal("timed out waiting for stun discovery to update identity")
	}

	if finalAddr.IP.String() != "203.0.113.50" {
		t.Errorf("expected IP %s, got %s", "203.0.113.50", finalAddr.IP.String())
	}
	if finalAddr.Port != 54321 {
		t.Errorf("expected port %d, got %d", 54321, finalAddr.Port)
	}
}
