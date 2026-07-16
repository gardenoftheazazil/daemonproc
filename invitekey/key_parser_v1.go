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

// KeyParserV1 is responsible for parsing, validating, and verifying invite keys.
type KeyParserV1 struct {
	prefix       string            // Prefix for version 1 keys, e.g., "gota1".
	verifyingKey ed25519.PublicKey // Ed25519 Public Key used to verify the payload signature.
}

// NewKeyParserV1 initializes and returns a new KeyParserV1 instance with the verification key.
func NewKeyParserV1(verifyingKey ed25519.PublicKey) *KeyParserV1 {
	return &KeyParserV1{
		prefix:       "gota1",
		verifyingKey: verifyingKey,
	}
}

// ParseKey parses a key string, verifies its signature, and returns the peer identity public key and address.
func (kp *KeyParserV1) ParseKey(keyStr string) (*ecdh.PublicKey, string, error) {
	// The prefix followed by '-' is 6 characters: "gota1-".
	prefixDash := kp.prefix + "-"
	if !strings.HasPrefix(keyStr, prefixDash) {
		return nil, "", fmt.Errorf("invalid key prefix")
	}

	// An Ed25519 signature is 64 bytes, and its Base64 Raw URL Encoding is exactly 86 characters.
	// Since the key is structured as: gota1-<payload>-<signature>
	// we expect the character before the signature to be '-'.
	// Minimum possible length: prefix(5) + "-" (1) +
	// payload (address "|" encoded_pubkey -> e.g. "a|43") +
	// "-" (1) + signature(86) = 95 characters.
	const minKeyLen = 95
	if len(keyStr) < minKeyLen {
		return nil, "", fmt.Errorf("invite key is too short")
	}

	// Split signature and payload from the end.
	sigStart := len(keyStr) - 86
	dashIdx := sigStart - 1
	if keyStr[dashIdx] != '-' {
		return nil, "", fmt.Errorf("invalid invite key format: missing signature separator")
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

	// Verify signature.
	if len(kp.verifyingKey) == 0 {
		return nil, "", fmt.Errorf("verifying key is empty")
	}
	if !ed25519.Verify(kp.verifyingKey, []byte(payload), sigBytes) {
		return nil, "", fmt.Errorf("signature verification failed")
	}

	// Parse payload: <address>|<encoded_public_key>
	// The encoded public key is Base64 Raw URL encoded, representing a 32-byte X25519 public key.
	// 32-byte Base64 Raw URL encoded key has length 43.
	// Let's locate the last '|' character.
	idx := strings.LastIndex(payload, "|")
	if idx == -1 {
		return nil, "", fmt.Errorf("invalid payload format: missing public key separator")
	}

	peerAddr := payload[:idx]
	if peerAddr == "" {
		return nil, "", fmt.Errorf("peer address is empty")
	}

	encodedPubKey := payload[idx+1:]
	pubKeyBytes, err := base64.RawURLEncoding.DecodeString(encodedPubKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode public key: %w", err)
	}

	peerIdentity, err := ecdh.X25519().NewPublicKey(pubKeyBytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse ecdh public key: %w", err)
	}

	return peerIdentity, peerAddr, nil
}
