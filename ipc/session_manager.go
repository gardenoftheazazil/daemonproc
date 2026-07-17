// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package ipc

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gardenoftheazazil/daemonproc/interfaces"
)

// maxPacketSize limits local IPC message payload sizes to avoid memory issues (32KB).
// 15 bits are used for length since the MSB of the 16-bit header is reserved for the control flag.
const maxPacketSize = 32767

// SessionState holds the configuration and active objects for an IPC session.
type SessionState struct {
	DID  interfaces.DID
	Conn net.Conn
}

// SessionManager implements interfaces.IIpcSessionManager.
type SessionManager struct {
	mu        sync.RWMutex
	sessions  map[interfaces.DID]*SessionState
	nextDID   atomic.Uint32
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	onClose   func(did interfaces.DID)
	egress    interfaces.IEgress
	onControl func(did interfaces.DID, payload []byte)
}

// NewSessionManager creates a new SessionManager instance.
func NewSessionManager(ctx context.Context, egress interfaces.IEgress) *SessionManager {
	childCtx, cancel := context.WithCancel(ctx)
	return &SessionManager{
		sessions: make(map[interfaces.DID]*SessionState),
		ctx:      childCtx,
		cancel:   cancel,
		egress:   egress,
	}
}

// SetOnClose sets a callback to be run when a session is closed.
func (sm *SessionManager) SetOnClose(callback func(did interfaces.DID)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onClose = callback
}

// SetControlCallback sets the callback function to handle incoming local control packets.
func (sm *SessionManager) SetControlCallback(cb interfaces.ControlCallback) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onControl = cb
}

// RegisterSession assigns a unique DID to a new local connection and tracks it.
func (sm *SessionManager) RegisterSession(conn net.Conn) (interfaces.DID, error) {
	if conn == nil {
		return 0, fmt.Errorf("connection cannot be nil")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Select a unique non-zero DID.
	didVal := sm.nextDID.Add(1)
	did := interfaces.DID(didVal)

	sess := &SessionState{
		DID:  did,
		Conn: conn,
	}
	sm.sessions[did] = sess

	// Spawn a read loop goroutine to handle incoming framed packets from this local connection.
	sm.wg.Add(1)
	go sm.readLoop(sess)

	return did, nil
}

// UnregisterSession removes a session by its DID and closes the connection.
func (sm *SessionManager) UnregisterSession(did interfaces.DID) error {
	sm.mu.Lock()
	sess, exists := sm.sessions[did]
	if !exists {
		sm.mu.Unlock()
		return fmt.Errorf("session ID %d not found", did)
	}
	delete(sm.sessions, did)
	onClose := sm.onClose
	sm.mu.Unlock()

	// Close the connection.
	err := sess.Conn.Close()

	if onClose != nil {
		onClose(did)
	}

	if err != nil {
		return fmt.Errorf("failed to close connection for DID %d: %w", did, err)
	}
	return nil
}

// SendToLocal routes decrypted network payloads to the specific local application.
func (sm *SessionManager) SendToLocal(did interfaces.DID, data []byte) error {
	sm.mu.RLock()
	sess, exists := sm.sessions[did]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session ID %d not found", did)
	}

	// Set write deadline to prevent slow clients from blocking the daemon.
	_ = sess.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	// Write length-prefixed packet.
	if err := sm.writePacket(sess.Conn, false, data); err != nil {
		return fmt.Errorf("failed to write payload to local client for DID %d: %w", did, err)
	}
	return nil
}

// SendControlToLocal sends a local control response to the specific local application.
func (sm *SessionManager) SendControlToLocal(did interfaces.DID, data []byte) error {
	sm.mu.RLock()
	sess, exists := sm.sessions[did]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session ID %d not found", did)
	}

	// Set write deadline to prevent slow clients from blocking the daemon.
	_ = sess.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	// Write control response packet.
	if err := sm.writePacket(sess.Conn, true, data); err != nil {
		return fmt.Errorf("failed to write control response to local client for DID %d: %w", did, err)
	}
	return nil
}

// RouteToNetwork handles payloads received from local apps and forwards them to Egress.
func (sm *SessionManager) RouteToNetwork(did interfaces.DID, data []byte) error {
	sm.mu.RLock()
	_, exists := sm.sessions[did]
	egress := sm.egress
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session ID %d not found", did)
	}

	if egress == nil {
		return fmt.Errorf("egress service not registered")
	}

	// Forward using Egress.RouteToNetwork.
	err := egress.RouteToNetwork(did, data)
	if err != nil {
		return fmt.Errorf("egress route failed for DID %d: %w", did, err)
	}

	return nil
}

// Close stops the session manager, cancelling all read loops and closing all active connections.
func (sm *SessionManager) Close() error {
	sm.cancel()

	sm.mu.Lock()
	activeSessions := make([]*SessionState, 0, len(sm.sessions))
	for _, sess := range sm.sessions {
		activeSessions = append(activeSessions, sess)
	}
	sm.sessions = make(map[interfaces.DID]*SessionState)
	sm.mu.Unlock()

	for _, sess := range activeSessions {
		_ = sess.Conn.Close()
	}

	sm.wg.Wait()
	return nil
}

func (sm *SessionManager) readLoop(sess *SessionState) {
	defer sm.wg.Done()

	// Ensure connection gets unregistered when the read loop terminates.
	defer func() {
		_ = sm.UnregisterSession(sess.DID)
	}()

	for {
		select {
		case <-sm.ctx.Done():
			return
		default:
		}

		isControl, payload, err := sm.readPacket(sess.Conn)
		if err != nil {
			return
		}

		if isControl {
			sm.mu.RLock()
			onCtrl := sm.onControl
			sm.mu.RUnlock()
			if onCtrl != nil {
				onCtrl(sess.DID, payload)
			}
		} else {
			_ = sm.RouteToNetwork(sess.DID, payload)
		}
	}
}

func (sm *SessionManager) readPacket(r io.Reader) (isControl bool, payload []byte, err error) {
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

func (sm *SessionManager) writePacket(w io.Writer, isControl bool, data []byte) error {
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
