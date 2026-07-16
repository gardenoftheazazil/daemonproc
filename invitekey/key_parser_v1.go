// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package invitekey

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"strings"
)

// KeyParserV1 verifies the self-signed key to ensure no relay node has tampered with it.
type KeyParserV1 struct {
	prefix string // Prefix for version 1 keys, e.g., "gota1".
}

// NewKeyParserV1 initializes and returns a new KeyParserV1 instance.
func NewKeyParserV1() *KeyParserV1 {
	return &KeyParserV1{
		prefix: "gota1",
	}
}

// ParseKey parses a self-signed key, verifies its signature against the contained Ed25519 key,
// and returns the ecdh.PublicKey (X25519) and the network address.
func (kp *KeyParserV1) ParseKey(keyStr string) (*ecdh.PublicKey, string, error) {
	// The prefix followed by '-' is 6 characters: "gota1-".
	prefixDash := kp.prefix + "-"
	if !strings.HasPrefix(keyStr, prefixDash) {
		return nil, "", fmt.Errorf("invalid key prefix")
	}

	// Minimum possible length: prefix(5) + "-" (1) + payload (address "|" X25519 "|" Ed25519) + "-" (1) + signature(86)
	// X25519 is 43 characters, Ed25519 is 43 characters. Minimum address is 1 character.
	// 5 + 1 + (1 + 1 + 43 + 1 + 43) + 1 + 86 = 182 characters.
	const minKeyLen = 182
	if len(keyStr) < minKeyLen {
		return nil, "", fmt.Errorf("invite key is too short")
	}

	sigStart := len(keyStr) - 86
	dashIdx := sigStart - 1
	if keyStr[dashIdx] != '-' {
		return nil, "", fmt.Errorf("invalid format: missing signature separator")
	}

	payload := keyStr[len(prefixDash):dashIdx]
	signature := keyStr[sigStart:]

	// Decode signature.
	sigBytes, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode signature: %w", err)
	}
	if len(sigBytes) != ed25519.SignatureSize {
		return nil, "", fmt.Errorf("invalid signature size: expected %d, got %d",
			ed25519.SignatureSize, len(sigBytes))
	}

	// Parse payload: <address>|<encodedX25519>|<encodedEd25519>
	// Split from the end to find encodedEd25519.
	idxEd := strings.LastIndex(payload, "|")
	if idxEd == -1 {
		return nil, "", fmt.Errorf("invalid payload: missing ed25519 key separator")
	}
	encodedEd25519 := payload[idxEd+1:]
	remainingPayload := payload[:idxEd]

	// Split again to find encodedX25519.
	idxX := strings.LastIndex(remainingPayload, "|")
	if idxX == -1 {
		return nil, "", fmt.Errorf("invalid payload: missing x25519 key separator")
	}
	encodedX25519 := remainingPayload[idxX+1:]
	peerAddr := remainingPayload[:idxX]

	if peerAddr == "" {
		return nil, "", fmt.Errorf("peer address is empty")
	}

	// Decode keys.
	ed25519Bytes, err := base64.RawURLEncoding.DecodeString(encodedEd25519)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode ed25519 key: %w", err)
	}
	if len(ed25519Bytes) != ed25519.PublicKeySize {
		return nil, "", fmt.Errorf("invalid ed25519 key size: expected %d, got %d",
			ed25519.PublicKeySize, len(ed25519Bytes))
	}

	x25519Bytes, err := base64.RawURLEncoding.DecodeString(encodedX25519)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode x25519 key: %w", err)
	}

	// Verify signature using the embedded Ed25519 public key.
	verifyingKey := ed25519.PublicKey(ed25519Bytes)
	if !ed25519.Verify(verifyingKey, []byte(payload), sigBytes) {
		return nil, "", fmt.Errorf("tampering detected: signature verification failed")
	}

	peerIdentity, err := ecdh.X25519().NewPublicKey(x25519Bytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse ecdh public key: %w", err)
	}

	return peerIdentity, peerAddr, nil
}
