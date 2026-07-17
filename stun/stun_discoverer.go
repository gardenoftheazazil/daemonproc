// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package stun

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/gardenoftheazazil/daemonproc/identify"
	"github.com/gardenoftheazazil/daemonproc/interfaces"
)

// Discoverer is responsible for discovering the public IP address
// and port of a client behind a NAT using the STUN protocol.
type Discoverer struct {
	mu           sync.Mutex
	portListener interfaces.IUdpPortListener
	servers      []string
	identity     *identify.Identity
	interval     time.Duration
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	isRunning    bool
}

// NewDiscoverer creates a new Discoverer instance with the given configuration.
func NewDiscoverer(
	portListener interfaces.IUdpPortListener,
	servers []string,
	identity *identify.Identity,
	interval time.Duration,
) *Discoverer {
	return &Discoverer{
		portListener: portListener,
		servers:      servers,
		identity:     identity,
		interval:     interval,
	}
}

// Start launches the background STUN discovery process.
// It returns an error if the process is already running.
func (d *Discoverer) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.isRunning {
		return fmt.Errorf("stun discoverer is already running")
	}

	if len(d.servers) == 0 {
		return fmt.Errorf("no stun servers configured")
	}

	d.isRunning = true
	childCtx, cancel := context.WithCancel(ctx)
	d.cancel = cancel

	d.wg.Add(1)
	go d.discoveryLoop(childCtx)

	return nil
}

// Stop stops the background STUN discovery process.
// It blocks until the background worker has terminated.
func (d *Discoverer) Stop() error {
	d.mu.Lock()
	if !d.isRunning {
		d.mu.Unlock()
		return nil
	}

	d.isRunning = false
	if d.cancel != nil {
		d.cancel()
	}
	d.mu.Unlock()

	d.wg.Wait()
	return nil
}

// discoveryLoop runs in a background goroutine and periodically queries the configured STUN servers.
func (d *Discoverer) discoveryLoop(ctx context.Context) {
	defer d.wg.Done()

	// Initial query happens immediately.
	d.queryAll(ctx)

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.queryAll(ctx)
		}
	}
}

// queryAll iterates through the list of configured STUN servers sequentially
// and attempts to discover the public IP/port until one succeeds.
func (d *Discoverer) queryAll(ctx context.Context) {
	for _, server := range d.servers {
		select {
		case <-ctx.Done():
			return
		default:
		}

		slog.Debug("attempting stun discovery", "server", server)
		addr, err := d.queryServer(ctx, server)
		if err == nil {
			slog.Info("successfully discovered public address via stun", "server", server, "address", addr.String())
			d.identity.SetPublicAddr(addr)
			return
		}
		slog.Warn("stun discovery failed for server", "server", server, "error", err.Error())
	}
	slog.Error("all configured stun servers failed to respond")
}

// queryServer queries a single STUN server and returns the mapped address.
func (d *Discoverer) queryServer(ctx context.Context, server string) (*net.UDPAddr, error) {
	// Resolve server address.
	serverAddr, err := net.ResolveUDPAddr("udp", server)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve stun server address: %w", err)
	}

	// Generate a unique 12-byte transaction ID.
	var txID [12]byte
	if _, randErr := rand.Read(txID[:]); randErr != nil {
		return nil, fmt.Errorf("failed to generate random transaction ID: %w", randErr)
	}

	// Construct standard STUN binding request packet.
	req := make([]byte, 20)
	binary.BigEndian.PutUint16(req[0:2], 0x0001)     // Message Type: Binding Request.
	binary.BigEndian.PutUint16(req[2:4], 0x0000)     // Message Length: 0.
	binary.BigEndian.PutUint32(req[4:8], 0x2112A442) // Magic Cookie.
	copy(req[8:20], txID[:])

	packetChan := make(chan []byte, 1)

	// Callback matches packets from the STUN server containing our transaction ID.
	callback := func(sourceAddr *net.UDPAddr, data []byte) bool {
		if sourceAddr.IP.String() != serverAddr.IP.String() || sourceAddr.Port != serverAddr.Port {
			return false
		}
		if len(data) < 20 {
			return false
		}
		// Match Magic Cookie and Transaction ID.
		magicCookie := binary.BigEndian.Uint32(data[4:8])
		if magicCookie != 0x2112A442 {
			return false
		}
		var msgTxID [12]byte
		copy(msgTxID[:], data[8:20])
		return msgTxID == txID
	}

	// Subscribe to packet listener.
	subID, err := d.portListener.SubscribeToPacketReceived(callback, packetChan)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to packet listener: %w", err)
	}
	defer func() {
		_ = d.portListener.UnsubscribeFromPacketReceived(subID)
	}()

	// Send STUN binding request.
	if err := d.portListener.SendPacketTo(serverAddr, req); err != nil {
		return nil, fmt.Errorf("failed to send stun request packet: %w", err)
	}

	// Wait for STUN response or timeout.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case packet := <-packetChan:
		return parseSTUNResponse(packet, txID)
	case <-time.After(2 * time.Second):
		return nil, fmt.Errorf("stun request timed out")
	}
}

// parseSTUNResponse parses the mapped address from a STUN Binding Success Response packet.
func parseSTUNResponse(data []byte, expectedTxID [12]byte) (*net.UDPAddr, error) {
	if len(data) < 20 {
		return nil, fmt.Errorf("stun response packet is too short")
	}

	msgType := binary.BigEndian.Uint16(data[0:2])
	msgLen := binary.BigEndian.Uint16(data[2:4])
	magicCookie := binary.BigEndian.Uint32(data[4:8])

	if msgType != 0x0101 { // Binding Success Response.
		return nil, fmt.Errorf("unexpected stun message type: 0x%04x", msgType)
	}
	if magicCookie != 0x2112A442 {
		return nil, fmt.Errorf("invalid stun magic cookie: 0x%08x", magicCookie)
	}

	var txID [12]byte
	copy(txID[:], data[8:20])
	if txID != expectedTxID {
		return nil, fmt.Errorf("stun transaction ID mismatch")
	}

	if len(data) < 20+int(msgLen) {
		return nil, fmt.Errorf("stun packet is truncated")
	}

	attrs := data[20 : 20+msgLen]
	idx := 0
	for idx+4 <= len(attrs) {
		attrType := binary.BigEndian.Uint16(attrs[idx : idx+2])
		attrLen := binary.BigEndian.Uint16(attrs[idx+2 : idx+4])
		valStart := idx + 4
		valEnd := valStart + int(attrLen)
		if valEnd > len(attrs) {
			return nil, fmt.Errorf("stun attribute length exceeds message boundaries")
		}

		val := attrs[valStart:valEnd]

		switch attrType {
		case 0x0020: // XOR-MAPPED-ADDRESS.
			addr, err := parseXORMappedAddress(val, txID)
			if err == nil {
				return addr, nil
			}
		case 0x0001: // MAPPED-ADDRESS.
			addr, err := parseMappedAddress(val)
			if err == nil {
				return addr, nil
			}
		}

		// Align to 4-byte boundary.
		paddedLen := (int(attrLen) + 3) &^ 3
		idx += 4 + paddedLen
	}

	return nil, fmt.Errorf("no mapped address attribute found in stun response")
}

// parseXORMappedAddress decodes the XOR-MAPPED-ADDRESS STUN attribute.
func parseXORMappedAddress(val []byte, txID [12]byte) (*net.UDPAddr, error) {
	if len(val) < 8 {
		return nil, fmt.Errorf("xor-mapped-address attribute is too short")
	}
	family := val[1]
	xPort := binary.BigEndian.Uint16(val[2:4])
	port := xPort ^ 0x2112

	switch family {
	case 0x01: // IPv4.
		xAddr := binary.BigEndian.Uint32(val[4:8])
		ipVal := xAddr ^ 0x2112A442
		ipBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(ipBytes, ipVal)
		return &net.UDPAddr{
			IP:   net.IP(ipBytes),
			Port: int(port),
		}, nil
	case 0x02: // IPv6.
		if len(val) < 20 {
			return nil, fmt.Errorf("xor-mapped-address is too short for ipv6")
		}
		ipBytes := make([]byte, 16)
		mask := make([]byte, 16)
		binary.BigEndian.PutUint32(mask[0:4], 0x2112A442)
		copy(mask[4:16], txID[:])
		for i := range 16 {
			ipBytes[i] = val[4+i] ^ mask[i]
		}
		return &net.UDPAddr{
			IP:   net.IP(ipBytes),
			Port: int(port),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported protocol family in xor-mapped-address: %d", family)
	}
}

// parseMappedAddress decodes the MAPPED-ADDRESS STUN attribute.
func parseMappedAddress(val []byte) (*net.UDPAddr, error) {
	if len(val) < 8 {
		return nil, fmt.Errorf("mapped-address attribute is too short")
	}
	family := val[1]
	port := binary.BigEndian.Uint16(val[2:4])

	switch family {
	case 0x01: // IPv4.
		ipBytes := make([]byte, 4)
		copy(ipBytes, val[4:8])
		return &net.UDPAddr{
			IP:   net.IP(ipBytes),
			Port: int(port),
		}, nil
	case 0x02: // IPv6.
		if len(val) < 20 {
			return nil, fmt.Errorf("mapped-address is too short for ipv6")
		}
		ipBytes := make([]byte, 16)
		copy(ipBytes, val[4:20])
		return &net.UDPAddr{
			IP:   net.IP(ipBytes),
			Port: int(port),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported protocol family in mapped-address: %d", family)
	}
}
