// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

// Package network provides the network listener and communication mechanisms.
package network

import (
	"context"
	"fmt"
	"net"
	"sync"
)

type subscription struct {
	callback func(sourceAddr *net.UDPAddr, data []byte) bool
	ch       chan []byte
}

// UDPListener implements interfaces.IUdpPortListener.
type UDPListener struct {
	mu            sync.RWMutex
	conn          *net.UDPConn
	subscriptions map[uint32]subscription
	nextSubID     uint32
	isListening   bool
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// NewUDPListener creates a new UDPListener instance.
func NewUDPListener() *UDPListener {
	return &UDPListener{
		subscriptions: make(map[uint32]subscription),
	}
}

// StartListener starts listening on the specified UDP port.
func (l *UDPListener) StartListener(ctx context.Context, port uint16) error {
	l.mu.Lock()
	if l.isListening {
		l.mu.Unlock()
		return fmt.Errorf("listener is already running")
	}

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		l.mu.Unlock()
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		l.mu.Unlock()
		return fmt.Errorf("failed to listen on UDP port %d: %w", port, err)
	}

	l.conn = conn
	l.isListening = true

	// Create a cancellable context linked to the parent context.
	childCtx, cancel := context.WithCancel(ctx)
	l.cancel = cancel
	l.mu.Unlock()

	l.wg.Add(1)
	go l.readLoop(childCtx, conn)

	return nil
}

// StopListener stops listening on the UDP port.
func (l *UDPListener) StopListener() error {
	l.mu.Lock()
	if !l.isListening {
		l.mu.Unlock()
		return nil
	}

	l.isListening = false
	if l.conn != nil {
		_ = l.conn.Close()
	}
	if l.cancel != nil {
		l.cancel()
	}
	l.mu.Unlock()

	l.wg.Wait()
	return nil
}

// SendPacketTo sends a UDP packet to the specified IP address and port with the given data.
func (l *UDPListener) SendPacketTo(targetAddr *net.UDPAddr, data []byte) error {
	l.mu.RLock()
	conn := l.conn
	isListening := l.isListening
	l.mu.RUnlock()

	if !isListening || conn == nil {
		return fmt.Errorf("listener is not running")
	}

	_, err := conn.WriteToUDP(data, targetAddr)
	if err != nil {
		return fmt.Errorf("failed to write to UDP: %w", err)
	}

	return nil
}

// SubscribeToPacketReceived allows subscribing to incoming UDP packets.
func (l *UDPListener) SubscribeToPacketReceived(
	callback func(sourceAddr *net.UDPAddr, data []byte) bool,
	packetChannel chan []byte,
) (uint32, error) {
	if callback == nil {
		return 0, fmt.Errorf("callback cannot be nil")
	}
	if packetChannel == nil {
		return 0, fmt.Errorf("packet channel cannot be nil")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.nextSubID++
	id := l.nextSubID
	l.subscriptions[id] = subscription{
		callback: callback,
		ch:       packetChannel,
	}

	return id, nil
}

// UnsubscribeFromPacketReceived allows unsubscribing from the packet received event.
func (l *UDPListener) UnsubscribeFromPacketReceived(subscriptionID uint32) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.subscriptions[subscriptionID]; !exists {
		return fmt.Errorf("subscription ID %d not found", subscriptionID)
	}

	delete(l.subscriptions, subscriptionID)
	return nil
}

func (l *UDPListener) readLoop(ctx context.Context, conn *net.UDPConn) {
	defer l.wg.Done()

	buf := make([]byte, 65535)
	for {
		// Check context before blocking.
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			// ReadFromUDP returns an error when the connection is closed.
			// Check if we are intentionally stopping.
			l.mu.RLock()
			isListening := l.isListening
			l.mu.RUnlock()
			if !isListening {
				return
			}
			continue
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		l.mu.RLock()
		// Copy subscriptions map to avoid holding the lock during callbacks.
		subs := make([]subscription, 0, len(l.subscriptions))
		for _, sub := range l.subscriptions {
			subs = append(subs, sub)
		}
		l.mu.RUnlock()

		for _, sub := range subs {
			// Call the callback in a separate goroutine as specified.
			go func(s subscription, sa *net.UDPAddr, d []byte) {
				if s.callback(sa, d) {
					select {
					case s.ch <- d:
					default:
						// Safely drop package if channel is full or blocked.
					}
				}
			}(sub, addr, data)
		}
	}
}
