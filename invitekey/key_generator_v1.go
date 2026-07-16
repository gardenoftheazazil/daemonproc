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

// KeyGeneratorV1 is responsible for generating keys for version 1 of the invite key format.
// It structures the peer address and x25519 public key as the payload, then signs it
// using an ed25519 private key to ensure data integrity (tamper-proofing), similar to a JWT.
type KeyGeneratorV1 struct {
	prefix       string             // Prefix for version 1 keys, e.g., "gota1".
	peerIdentity *ecdh.PublicKey    // Peer identity public key (pointer to ecdh.PublicKey).
	peerAddr     string             // Peer address used in the key generation process.
	signingKey   ed25519.PrivateKey // Ed25519 Private Key used to sign the payload for integrity.
}

// NewKeyGeneratorV1 initializes and returns a new KeyGeneratorV1 instance with the signing key.
func NewKeyGeneratorV1(peerIdentity *ecdh.PublicKey, peerAddr string, signingKey ed25519.PrivateKey) *KeyGeneratorV1 {
	return &KeyGeneratorV1{
		prefix:       "gota1",
		peerIdentity: peerIdentity,
		peerAddr:     peerAddr,
		signingKey:   signingKey,
	}
}

// GenerateKey constructs the complete invite key string by formatting and signing the components.
// The output format is: gota1-<payload>-<signature> where <payload> is: <address>|<encoded_public_key>.
func (kg *KeyGeneratorV1) GenerateKey() (string, error) {
	// Validate inputs before processing.
	if kg.peerAddr == "" {
		return "", fmt.Errorf("peer address is empty")
	}
	if kg.peerIdentity == nil {
		return "", fmt.Errorf("peer identity public key is nil")
	}
	if len(kg.signingKey) == 0 {
		return "", fmt.Errorf("signing key is empty")
	}

	var keyBuilder strings.Builder

	// 1. Write the prefix followed by the separator (e.g., "gota1-").
	keyBuilder.WriteString(kg.prefix)
	keyBuilder.WriteByte('-')

	// 2. Encode the binary public key using Base64 Raw URL Encoding.
	pubKeyBytes := kg.peerIdentity.Bytes()
	encodedPubKey := base64.RawURLEncoding.EncodeToString(pubKeyBytes)

	// 3. Construct the payload (e.g., "addr|encoded_key").
	var payloadBuilder strings.Builder
	payloadBuilder.WriteString(kg.peerAddr)
	payloadBuilder.WriteByte('|')
	payloadBuilder.WriteString(encodedPubKey)
	payload := payloadBuilder.String()

	// 4. Append the payload to the main key.
	keyBuilder.WriteString(payload)
	keyBuilder.WriteByte('-')

	// 5. Generate signature of the payload and append it.
	signature, err := kg.signKeyData([]byte(payload))
	if err != nil {
		return "", fmt.Errorf("failed to sign key data: %w", err)
	}
	keyBuilder.WriteString(signature)

	return keyBuilder.String(), nil
}

// signKeyData signs the structured payload using Ed25519 and returns a Base64 Raw URL encoded signature.
// This guarantees that the address and public key cannot be tampered with.
func (kg *KeyGeneratorV1) signKeyData(payload []byte) (string, error) {
	// Sign the payload directly using the Ed25519 private key.
	sigBytes := ed25519.Sign(kg.signingKey, payload)

	// Encode the signature to a safe string format.
	return base64.RawURLEncoding.EncodeToString(sigBytes), nil
}
