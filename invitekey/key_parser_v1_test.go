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
	_, signPriv, err := ed25519.GenerateKey(rand.Reader)
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
	parser := NewKeyParserV1()
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
	parser := NewKeyParserV1()

	tests := []struct {
		name    string
		key     string
		wantErr string
	}{
		{
			name:    "invalid prefix",
			key:     "gotax-abc",
			wantErr: "invalid key prefix",
		},
		{
			name:    "too short",
			key:     "gota1-short",
			wantErr: "invite key is too short",
		},
		{
			name:    "missing signature separator",
			key:     "gota1-" + strings.Repeat("a", 100) + "_" + strings.Repeat("a", 86),
			wantErr: "invalid format: missing signature separator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parser.ParseKey(tt.key)
			if err == nil {
				t.Errorf("ParseKey() expected error containing %q, got nil", tt.wantErr)
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ParseKey() error = %v, expected to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestTamperingScenarios(t *testing.T) {
	// Setup keys.
	peerPriv, _ := ecdh.X25519().GenerateKey(rand.Reader)
	peerPub := peerPriv.PublicKey()
	_, signPriv, _ := ed25519.GenerateKey(rand.Reader)

	// Valid initial key.
	peerAddr := "127.0.0.1:9090"
	generator := NewKeyGeneratorV1(peerPub, peerAddr, signPriv)
	validKey, err := generator.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate valid key: %v", err)
	}

	parser := NewKeyParserV1()

	// Parse elements of validKey: gota1-<payload>-<sig>
	// Use length-based offsets to avoid splitting issues if Base64 URL strings contain dashes.
	sigStart := len(validKey) - 86
	payload := validKey[6 : sigStart-1]
	sig := validKey[sigStart:]

	// Payload structure: <address>|<x25519>|<ed25519>.
	payParts := strings.Split(payload, "|")
	if len(payParts) != 3 {
		t.Fatalf("unexpected payload structure: %s", payload)
	}
	addrPart := payParts[0]
	x25519Part := payParts[1]
	ed25519Part := payParts[2]

	t.Run("manipulated X25519 identity", func(t *testing.T) {
		// Generate an alternative X25519 key.
		altPriv, _ := ecdh.X25519().GenerateKey(rand.Reader)
		altPubEncoded := base64.RawURLEncoding.EncodeToString(altPriv.PublicKey().Bytes())

		// Rebuild payload with altered X25519 key, but keeping original signature.
		manipulatedPayload := addrPart + "|" + altPubEncoded + "|" + ed25519Part
		manipulatedKey := "gota1-" + manipulatedPayload + "-" + sig

		_, _, err := parser.ParseKey(manipulatedKey)
		if err == nil || !strings.Contains(err.Error(), "signature verification failed") {
			t.Errorf("expected signature verification failure, got: %v", err)
		}
	})

	t.Run("manipulated address/route", func(t *testing.T) {
		// Alter the address part of the payload.
		manipulatedPayload := "192.168.1.1:8080|" + x25519Part + "|" + ed25519Part
		manipulatedKey := "gota1-" + manipulatedPayload + "-" + sig

		_, _, err := parser.ParseKey(manipulatedKey)
		if err == nil || !strings.Contains(err.Error(), "signature verification failed") {
			t.Errorf("expected signature verification failure, got: %v", err)
		}
	})

	t.Run("manipulated signature", func(t *testing.T) {
		// Alter the signature by changing one character.
		var mutatedSig string
		if sig[0] == 'A' {
			mutatedSig = "B" + sig[1:]
		} else {
			mutatedSig = "A" + sig[1:]
		}
		manipulatedKey := "gota1-" + payload + "-" + mutatedSig

		_, _, err := parser.ParseKey(manipulatedKey)
		if err == nil || !strings.Contains(err.Error(), "signature verification failed") {
			t.Errorf("expected signature verification failure, got: %v", err)
		}
	})

	t.Run("manipulated verifying key", func(t *testing.T) {
		// Alter the verifying Ed25519 public key.
		_, altSignPriv, _ := ed25519.GenerateKey(rand.Reader)
		altEdEncoded := base64.RawURLEncoding.EncodeToString(altSignPriv.Public().(ed25519.PublicKey))

		manipulatedPayload := addrPart + "|" + x25519Part + "|" + altEdEncoded
		manipulatedKey := "gota1-" + manipulatedPayload + "-" + sig

		_, _, err := parser.ParseKey(manipulatedKey)
		if err == nil || !strings.Contains(err.Error(), "signature verification failed") {
			t.Errorf("expected signature verification failure, got: %v", err)
		}
	})
}

func FuzzParseKey(f *testing.F) {
	// Generate valid seed corpus.
	peerPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		f.Fatalf("failed to generate ecdh key: %v", err)
	}
	peerPub := peerPriv.PublicKey()

	_, signPriv, err := ed25519.GenerateKey(rand.Reader)
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
	f.Add("gota1-127.0.0.1:9090|abcdefg|hijklmn-invalid_sig")
	f.Add("invalid_prefix")
	f.Add("")

	parser := NewKeyParserV1()

	f.Fuzz(func(t *testing.T, keyStr string) {
		// Ensure parser does not panic on any fuzzed input.
		_, _, _ = parser.ParseKey(keyStr)
	})
}
