// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package identify

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"fmt"
	"net"
	"sync"
)

// Identity manages the local node's cryptographic keys and public-facing address.
type Identity struct {
	mu          sync.RWMutex
	publicAddr  *net.UDPAddr
	x25519Priv  *ecdh.PrivateKey
	x25519Pub   *ecdh.PublicKey
	ed25519Priv ed25519.PrivateKey
	ed25519Pub  ed25519.PublicKey
}

// NewIdentity initializes a new Identity with the provided private keys.
// Returns an error if any of the keys are nil or invalid.
func NewIdentity(x25519Priv *ecdh.PrivateKey, ed25519Priv ed25519.PrivateKey) (*Identity, error) {
	if x25519Priv == nil {
		return nil, fmt.Errorf("x25519 private key cannot be nil")
	}
	if len(ed25519Priv) == 0 {
		return nil, fmt.Errorf("ed25519 private key cannot be empty")
	}

	x25519Pub := x25519Priv.PublicKey()

	pub, ok := ed25519Priv.Public().(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("failed to derive ed25519 public key")
	}

	return &Identity{
		x25519Priv:  x25519Priv,
		x25519Pub:   x25519Pub,
		ed25519Priv: ed25519Priv,
		ed25519Pub:  pub,
	}, nil
}

// GetPublicAddr returns the node's current public-facing UDP address.
func (id *Identity) GetPublicAddr() *net.UDPAddr {
	id.mu.RLock()
	defer id.mu.RUnlock()
	if id.publicAddr == nil {
		return nil
	}
	// Return a copy of the UDPAddr to prevent external modification.
	return &net.UDPAddr{
		IP:   append([]byte(nil), id.publicAddr.IP...),
		Port: id.publicAddr.Port,
		Zone: id.publicAddr.Zone,
	}
}

// SetPublicAddr updates the node's public-facing UDP address thread-safely.
func (id *Identity) SetPublicAddr(addr *net.UDPAddr) {
	id.mu.Lock()
	defer id.mu.Unlock()
	if addr == nil {
		id.publicAddr = nil
		return
	}
	// Store a copy of UDPAddr.
	id.publicAddr = &net.UDPAddr{
		IP:   append([]byte(nil), addr.IP...),
		Port: addr.Port,
		Zone: addr.Zone,
	}
}

// GetX25519Keys returns the X25519 private and public keys.
func (id *Identity) GetX25519Keys() (*ecdh.PrivateKey, *ecdh.PublicKey) {
	id.mu.RLock()
	defer id.mu.RUnlock()
	return id.x25519Priv, id.x25519Pub
}

// GetEd25519Keys returns the Ed25519 private and public keys.
func (id *Identity) GetEd25519Keys() (ed25519.PrivateKey, ed25519.PublicKey) {
	id.mu.RLock()
	defer id.mu.RUnlock()
	return id.ed25519Priv, id.ed25519Pub
}
