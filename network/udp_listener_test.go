// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package network

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestUDPListenerStartStop(t *testing.T) {
	l := NewUDPListener()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start listener on a random port (port 0 lets the OS pick a free port).
	err := l.StartListener(ctx, 0)
	if err != nil {
		t.Fatalf("StartListener failed: %v", err)
	}

	// Verify listening status.
	l.mu.RLock()
	isListening := l.isListening
	conn := l.conn
	l.mu.RUnlock()

	if !isListening {
		t.Error("expected listener to be active")
	}
	if conn == nil {
		t.Error("expected connection to be initialized")
	}

	// Stop listener.
	err = l.StopListener()
	if err != nil {
		t.Fatalf("StopListener failed: %v", err)
	}

	l.mu.RLock()
	isListening = l.isListening
	l.mu.RUnlock()

	if isListening {
		t.Error("expected listener to be inactive after stopping")
	}
}

func TestUDPListenerSendReceive(t *testing.T) {
	l := NewUDPListener()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := l.StartListener(ctx, 0)
	if err != nil {
		t.Fatalf("StartListener failed: %v", err)
	}
	defer l.StopListener()

	l.mu.RLock()
	localAddr := l.conn.LocalAddr().(*net.UDPAddr)
	l.mu.RUnlock()

	targetAddr := &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: localAddr.Port,
	}

	packetChan := make(chan []byte, 10)
	callbackCalled := make(chan struct{}, 1)

	// Subscribe.
	subID, err := l.SubscribeToPacketReceived(func(sourceAddr *net.UDPAddr, data []byte) bool {
		callbackCalled <- struct{}{}
		return string(data) == "hello"
	}, packetChan)
	if err != nil {
		t.Fatalf("SubscribeToPacketReceived failed: %v", err)
	}

	// Send packet to listener from a temporary client.
	clientConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer clientConn.Close()

	payload1 := []byte("hello")
	_, err = clientConn.WriteToUDP(payload1, targetAddr)
	if err != nil {
		t.Fatalf("WriteToUDP failed: %v", err)
	}

	// Wait for callback and channel read.
	select {
	case <-callbackCalled:
		// Callback was triggered.
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for callback")
	}

	select {
	case data := <-packetChan:
		if string(data) != "hello" {
			t.Errorf("expected %q, got %q", "hello", string(data))
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for packet in channel")
	}

	// Send packet that doesn't match callback condition (returns false).
	payload2 := []byte("world")
	_, err = clientConn.WriteToUDP(payload2, targetAddr)
	if err != nil {
		t.Fatalf("WriteToUDP failed: %v", err)
	}

	select {
	case <-callbackCalled:
		// Callback was triggered again.
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for callback on second packet")
	}

	select {
	case data := <-packetChan:
		t.Fatalf("unexpected packet received in channel: %s", string(data))
	case <-time.After(100 * time.Millisecond):
		// Success - packet was discarded by callback returning false.
	}

	// Unsubscribe.
	err = l.UnsubscribeFromPacketReceived(subID)
	if err != nil {
		t.Fatalf("UnsubscribeFromPacketReceived failed: %v", err)
	}

	// Send another packet - shouldn't trigger anything.
	_, err = clientConn.WriteToUDP(payload1, targetAddr)
	if err != nil {
		t.Fatalf("WriteToUDP failed: %v", err)
	}

	select {
	case <-callbackCalled:
		t.Fatal("callback triggered after unsubscribing")
	case <-time.After(200 * time.Millisecond):
		// Success.
	}
}

func TestUDPListenerDropOnBlock(t *testing.T) {
	l := NewUDPListener()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := l.StartListener(ctx, 0)
	if err != nil {
		t.Fatalf("StartListener failed: %v", err)
	}
	defer l.StopListener()

	l.mu.RLock()
	localAddr := l.conn.LocalAddr().(*net.UDPAddr)
	l.mu.RUnlock()

	targetAddr := &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: localAddr.Port,
	}

	// Channel with buffer size 1.
	packetChan := make(chan []byte, 1)

	_, err = l.SubscribeToPacketReceived(func(sourceAddr *net.UDPAddr, data []byte) bool {
		return true
	}, packetChan)
	if err != nil {
		t.Fatalf("SubscribeToPacketReceived failed: %v", err)
	}

	clientConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer clientConn.Close()

	// Send 3 packets.
	for range 3 {
		_, err = clientConn.WriteToUDP([]byte("ping"), targetAddr)
		if err != nil {
			t.Fatalf("WriteToUDP failed: %v", err)
		}
	}

	// Wait for network propagation.
	time.Sleep(100 * time.Millisecond)

	// We should be able to read 1 packet from the channel.
	// The other 2 packets should have been dropped safely instead of blocking or causing a panic.
	select {
	case <-packetChan:
		// Read successfully.
	default:
		t.Fatal("expected at least one packet in channel")
	}

	select {
	case <-packetChan:
		t.Fatal("unexpected second packet (should have been dropped due to channel block)")
	default:
		// Correct behavior: channel is empty because others were dropped.
	}
}
