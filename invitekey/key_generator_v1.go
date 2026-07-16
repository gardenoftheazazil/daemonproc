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

// KeyGeneratorV1 generates self-signed invite keys to guarantee secure,
// tamper-proof routing across untrusted relay peers in the mesh network.
type KeyGeneratorV1 struct {
	prefix       string             // Prefix for version 1 keys, e.g., "gota1".
	peerIdentity *ecdh.PublicKey    // X25519 Public Key for DH handshake.
	peerAddr     string             // Target address (or relay path).
	signingKey   ed25519.PrivateKey // Ed25519 Private Key of the generator itself.
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

// GenerateKey constructs the complete self-signed invite key string.
// Format: gota1-<peerAddr>|<encodedX25519>|<encodedEd25519>-<signature>.
func (kg *KeyGeneratorV1) GenerateKey() (string, error) {
	if kg.peerAddr == "" {
		return "", fmt.Errorf("peer address is empty")
	}
	if kg.peerIdentity == nil {
		return "", fmt.Errorf("peer identity public key is nil")
	}
	if len(kg.signingKey) == 0 {
		return "", fmt.Errorf("signing private key is empty")
	}

	var keyBuilder strings.Builder

	// 1. Add prefix followed by the separator (e.g., "gota1-").
	keyBuilder.WriteString(kg.prefix)
	keyBuilder.WriteByte('-')

	// 2. Encode the X25519 binary public key using Base64 Raw URL Encoding.
	pubKeyBytes := kg.peerIdentity.Bytes()
	encodedPubKey := base64.RawURLEncoding.EncodeToString(pubKeyBytes)

	// 3. Encode the Ed25519 verifying public key (derived from private key).
	verifyingPubKey, ok := kg.signingKey.Public().(ed25519.PublicKey)
	if !ok {
		return "", fmt.Errorf("failed to derive ed25519 public key")
	}
	encodedVerifyingKey := base64.RawURLEncoding.EncodeToString(verifyingPubKey)

	// 4. Construct payload (address + X25519 + Ed25519).
	var payloadBuilder strings.Builder
	payloadBuilder.WriteString(kg.peerAddr)
	payloadBuilder.WriteByte('|')
	payloadBuilder.WriteString(encodedPubKey)
	payloadBuilder.WriteByte('|')
	payloadBuilder.WriteString(encodedVerifyingKey)
	payload := payloadBuilder.String()

	// 5. Append payload to main key.
	keyBuilder.WriteString(payload)
	keyBuilder.WriteByte('-')

	// 6. Sign the payload using the generator's Ed25519 Private Key.
	sigBytes := ed25519.Sign(kg.signingKey, []byte(payload))
	encodedSignature := base64.RawURLEncoding.EncodeToString(sigBytes)
	keyBuilder.WriteString(encodedSignature)

	return keyBuilder.String(), nil
}
