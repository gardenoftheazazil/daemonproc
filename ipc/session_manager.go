// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package ipc

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gardenoftheazazil/daemonproc/interfaces"
)

// ErrSessionBlocked is returned when an IPC session's outbound queue is full (slow client).
var ErrSessionBlocked = errors.New("session outbound queue full (slow client)")

const (
	// maxPacketSize limits local IPC message payload sizes to avoid memory issues (64KB).
	maxPacketSize = 65535

	// sessionQueueSize specifies the maximum buffered outbound packets per session.
	sessionQueueSize = 1024
)

type outboundPacket struct {
	opcode     uint16
	statusCode uint16
	data       []byte
}

// SessionState holds the configuration and active objects for an IPC session.
type SessionState struct {
	DID       interfaces.DID
	Conn      net.Conn
	outbound  chan outboundPacket
	writeMu   sync.Mutex
	isClosed  bool
	closeOnce sync.Once
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
	onControl interfaces.ControlCallback
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

// SetControlCallback sets the callback function to handle incoming local IPC frames.
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
		DID:      did,
		Conn:     conn,
		outbound: make(chan outboundPacket, sessionQueueSize),
	}
	sm.sessions[did] = sess

	// Spawn read and write loop goroutines to handle asynchronous I/O for this local connection.
	sm.wg.Add(2)
	go sm.readLoop(sess)
	go sm.writeLoop(sess)

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

	var closeErr error
	sess.closeOnce.Do(func() {
		sess.writeMu.Lock()
		if !sess.isClosed {
			sess.isClosed = true
			close(sess.outbound)
		}
		sess.writeMu.Unlock()

		closeErr = sess.Conn.Close()
		if onClose != nil {
			onClose(did)
		}
	})

	if closeErr != nil {
		return fmt.Errorf("failed to close connection for DID %d: %w", did, closeErr)
	}
	return nil
}

// SendToLocal routes control or network payloads to the specific local application.
func (sm *SessionManager) SendToLocal(did interfaces.DID, opcode, statusCode uint16, data []byte) error {
	sm.mu.RLock()
	sess, exists := sm.sessions[did]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session ID %d not found", did)
	}

	sess.writeMu.Lock()
	if sess.isClosed {
		sess.writeMu.Unlock()
		return fmt.Errorf("session ID %d closed", did)
	}

	pkt := outboundPacket{
		opcode:     opcode,
		statusCode: statusCode,
		data:       data,
	}

	select {
	case sess.outbound <- pkt:
		sess.writeMu.Unlock()
		return nil
	default:
		sess.isClosed = true
		close(sess.outbound)
		sess.writeMu.Unlock()

		// Outbound queue is full! Unresponsive or slow client detected.
		go func() {
			_ = sm.UnregisterSession(did)
		}()
		return ErrSessionBlocked
	}
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

func (sm *SessionManager) writeLoop(sess *SessionState) {
	defer sm.wg.Done()

	defer func() {
		_ = sm.UnregisterSession(sess.DID)
	}()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case pkt, ok := <-sess.outbound:
			if !ok {
				return
			}
			_ = sess.Conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if err := sm.writePacket(sess.Conn, pkt.opcode, pkt.statusCode, pkt.data); err != nil {
				return
			}
		}
	}
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

		opcode, payload, err := sm.readPacket(sess.Conn)
		if err != nil {
			return
		}

		sm.mu.RLock()
		onCtrl := sm.onControl
		sm.mu.RUnlock()
		if onCtrl != nil {
			onCtrl(sess.DID, opcode, payload)
		}
	}
}

func (sm *SessionManager) readPacket(r io.Reader) (opcode uint16, payload []byte, err error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}

	opcode = binary.BigEndian.Uint16(header[0:2])
	length := binary.BigEndian.Uint16(header[2:4])

	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, nil, err
	}
	return opcode, buf, nil
}

func (sm *SessionManager) writePacket(w io.Writer, opcode, statusCode uint16, data []byte) error {
	length := len(data)
	if length > maxPacketSize {
		return fmt.Errorf("packet length %d exceeds maximum of %d", length, maxPacketSize)
	}

	var header [6]byte
	binary.BigEndian.PutUint16(header[0:2], opcode)
	binary.BigEndian.PutUint16(header[2:4], statusCode)
	binary.BigEndian.PutUint16(header[4:6], uint16(length))

	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	if length > 0 {
		_, err := w.Write(data)
		return err
	}
	return nil
}
