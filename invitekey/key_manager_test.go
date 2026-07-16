// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package invitekey

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
)

func TestKeyManagerRoundTrip(t *testing.T) {
	// 1. Setup keys.
	peerPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ecdh key: %v", err)
	}
	peerPub := peerPriv.PublicKey()

	_, signPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}

	peerAddr := "10.0.0.1:4182"

	// 2. Initialize KeyManager.
	km := NewKeyManager()

	// 3. Register KeyGeneratorV1 for v1keytype.
	generator := NewKeyGeneratorV1(peerPub, peerAddr, signPriv)
	km.RegisterGenerator(v1keytype, generator)

	// 4. Generate key via KeyManager.
	keyStr, err := km.GenerateKey(v1keytype)
	if err != nil {
		t.Fatalf("failed to generate key via KeyManager: %v", err)
	}

	// 5. Parse key via KeyManager (should auto-initialize parser for v1keytype/gota1).
	parsedPub, parsedAddr, err := km.ParseKey(keyStr)
	if err != nil {
		t.Fatalf("failed to parse key via KeyManager: %v", err)
	}

	// 6. Verify parsed values.
	if parsedAddr != peerAddr {
		t.Errorf("expected address %q, got %q", peerAddr, parsedAddr)
	}

	if !parsedPub.Equal(peerPub) {
		t.Errorf("parsed public key does not match original")
	}
}

func TestKeyManagerErrors(t *testing.T) {
	km := NewKeyManager()

	// Test GenerateKey with unregistered version.
	_, err := km.GenerateKey(999)
	if err == nil || !strings.Contains(err.Error(), "key generator not registered for version") {
		t.Errorf("expected error for unregistered generator version, got: %v", err)
	}

	// Test ParseKey with invalid format (no prefix).
	_, _, err = km.ParseKey("invalidkey")
	if err == nil || !strings.Contains(err.Error(), "invalid key format") {
		t.Errorf("expected error for missing prefix separator, got: %v", err)
	}

	// Test ParseKey with unsupported prefix.
	_, _, err = km.ParseKey("gota999-somepayload")
	if err == nil || !strings.Contains(err.Error(), "unsupported key prefix") {
		t.Errorf("expected error for unsupported prefix, got: %v", err)
	}
}
