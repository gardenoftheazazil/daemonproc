// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package invitekey

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	// Generate ECDH peer identity keys.
	peerPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ecdh key: %v", err)
	}
	peerPub := peerPriv.PublicKey()

	// Generate Ed25519 signing keys.
	signPub, signPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}

	peerAddr := "192.168.1.100:8080"

	// 1. Generate key.
	generator := NewKeyGeneratorV1(peerPub, peerAddr, signPriv)
	keyStr, err := generator.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// 2. Parse key.
	parser := NewKeyParserV1(signPub)
	parsedPub, parsedAddr, err := parser.ParseKey(keyStr)
	if err != nil {
		t.Fatalf("failed to parse key: %v", err)
	}

	// 3. Verify values.
	if parsedAddr != peerAddr {
		t.Errorf("expected address %q, got %q", peerAddr, parsedAddr)
	}

	if !parsedPub.Equal(peerPub) {
		t.Errorf("parsed public key does not match original")
	}
}

func TestKeyGeneratorValidation(t *testing.T) {
	peerPriv, _ := ecdh.X25519().GenerateKey(rand.Reader)
	peerPub := peerPriv.PublicKey()
	_, signPriv, _ := ed25519.GenerateKey(rand.Reader)

	tests := []struct {
		name       string
		peerAddr   string
		peerPub    *ecdh.PublicKey
		signingKey ed25519.PrivateKey
		wantErr    bool
	}{
		{
			name:       "valid input",
			peerAddr:   "127.0.0.1:9000",
			peerPub:    peerPub,
			signingKey: signPriv,
			wantErr:    false,
		},
		{
			name:       "empty address",
			peerAddr:   "",
			peerPub:    peerPub,
			signingKey: signPriv,
			wantErr:    true,
		},
		{
			name:       "nil peer identity",
			peerAddr:   "127.0.0.1:9000",
			peerPub:    nil,
			signingKey: signPriv,
			wantErr:    true,
		},
		{
			name:       "empty signing key",
			peerAddr:   "127.0.0.1:9000",
			peerPub:    peerPub,
			signingKey: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := NewKeyGeneratorV1(tt.peerPub, tt.peerAddr, tt.signingKey)
			_, err := generator.GenerateKey()
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestKeyParserErrors(t *testing.T) {
	peerPriv, _ := ecdh.X25519().GenerateKey(rand.Reader)
	peerPub := peerPriv.PublicKey()
	signPub, signPriv, _ := ed25519.GenerateKey(rand.Reader)
	otherSignPub, _, _ := ed25519.GenerateKey(rand.Reader)

	generator := NewKeyGeneratorV1(peerPub, "10.0.0.1:5000", signPriv)
	validKey, err := generator.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate valid key: %v", err)
	}

	parser := NewKeyParserV1(signPub)

	tests := []struct {
		name    string
		key     string
		parser  *KeyParserV1
		wantErr string
	}{
		{
			name:    "invalid prefix",
			key:     "gotax-abc",
			parser:  parser,
			wantErr: "invalid key prefix",
		},
		{
			name:    "too short",
			key:     "gota1-short",
			parser:  parser,
			wantErr: "invite key is too short",
		},
		{
			name:    "missing signature separator",
			key:     validKey[:len(validKey)-87] + "_" + validKey[len(validKey)-86:], // replace separator '-' with '_'.
			parser:  parser,
			wantErr: "invalid invite key format: missing signature separator",
		},
		{
			name:    "signature verification failed with other key",
			key:     validKey,
			parser:  NewKeyParserV1(otherSignPub),
			wantErr: "signature verification failed",
		},
		{
			name:    "corrupted signature (invalid base64 character)",
			key:     validKey[:len(validKey)-1] + "%",
			parser:  parser,
			wantErr: "failed to decode signature",
		},
		{
			name: "missing public key separator in payload",
			key: func() string {
				// Reconstruct key with payload that has no '|'.
				payload := "10.0.0.1_encodedkey"
				sigBytes := ed25519.Sign(signPriv, []byte(payload))
				sig := base64.RawURLEncoding.EncodeToString(sigBytes)
				return "gota1-" + payload + "-" + sig
			}(),
			parser:  parser,
			wantErr: "invalid payload format: missing public key separator",
		},
		{
			name: "empty address",
			key: func() string {
				// Reconstruct key with empty address before '|'.
				payload := "|encodedkey"
				sigBytes := ed25519.Sign(signPriv, []byte(payload))
				sig := base64.RawURLEncoding.EncodeToString(sigBytes)
				return "gota1-" + payload + "-" + sig
			}(),
			parser:  parser,
			wantErr: "peer address is empty",
		},
		{
			name: "corrupted public key base64",
			key: func() string {
				// Reconstruct key with bad pubkey encoding.
				payload := "10.0.0.1|invalid%base64"
				sigBytes := ed25519.Sign(signPriv, []byte(payload))
				sig := base64.RawURLEncoding.EncodeToString(sigBytes)
				return "gota1-" + payload + "-" + sig
			}(),
			parser:  parser,
			wantErr: "failed to decode public key",
		},
		{
			name: "invalid public key size",
			key: func() string {
				// Reconstruct key with short pubkey.
				shortPub := base64.RawURLEncoding.EncodeToString([]byte("shortkey"))
				payload := "10.0.0.1|" + shortPub
				sigBytes := ed25519.Sign(signPriv, []byte(payload))
				sig := base64.RawURLEncoding.EncodeToString(sigBytes)
				return "gota1-" + payload + "-" + sig
			}(),
			parser:  parser,
			wantErr: "failed to parse ecdh public key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tt.parser.ParseKey(tt.key)
			if err == nil {
				t.Errorf("ParseKey() expected error containing %q, got nil", tt.wantErr)
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ParseKey() error = %v, expected to contain %q", err, tt.wantErr)
			}
		})
	}
}

func FuzzParseKey(f *testing.F) {
	// Generate valid seed corpus.
	peerPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		f.Fatalf("failed to generate ecdh key: %v", err)
	}
	peerPub := peerPriv.PublicKey()

	signPub, signPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		f.Fatalf("failed to generate ed25519 key: %v", err)
	}

	generator := NewKeyGeneratorV1(peerPub, "127.0.0.1:9090", signPriv)
	validKey, err := generator.GenerateKey()
	if err != nil {
		f.Fatalf("failed to generate valid key: %v", err)
	}

	// Add seeds.
	f.Add(validKey)
	f.Add("gota1-127.0.0.1:9090|abcdefg-invalid_sig")
	f.Add("invalid_prefix")
	f.Add("")

	parser := NewKeyParserV1(signPub)

	f.Fuzz(func(t *testing.T, keyStr string) {
		// Ensure parser does not panic on any fuzzed input.
		_, _, _ = parser.ParseKey(keyStr)
	})
}
