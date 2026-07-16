// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package invitekey

import (
	"crypto/ecdh"
	"fmt"
	"strings"

	"github.com/gardenoftheazazil/daemonproc/interfaces"
)

const (
	v1keytype = iota
)

// KeyManager manages different versions of key generators and parsers for self-signed invite keys.
// KeyManager allows for the registration and retrieval of key generators and parsers based on version identifiers.
type KeyManager struct {
	generatorList   map[uint32]interfaces.IKeyGenerator
	ecdhParserList  map[uint32]interfaces.IKeyParser[*ecdh.PublicKey]
	prefixToVersion map[string]uint32
}

// NewKeyManager initializes and returns a new KeyManager instance.
func NewKeyManager() *KeyManager {
	return &KeyManager{
		generatorList:  make(map[uint32]interfaces.IKeyGenerator),
		ecdhParserList: make(map[uint32]interfaces.IKeyParser[*ecdh.PublicKey]),
		prefixToVersion: map[string]uint32{
			"gota1": v1keytype,
		},
	}
}

// RegisterGenerator registers a key generator for a given version.
func (km *KeyManager) RegisterGenerator(version uint32, generator interfaces.IKeyGenerator) {
	km.generatorList[version] = generator
}

// RegisterParser registers a key parser for a given version and prefix.
func (km *KeyManager) RegisterParser(version uint32, prefix string, parser interfaces.IKeyParser[*ecdh.PublicKey]) {
	km.ecdhParserList[version] = parser
	km.prefixToVersion[prefix] = version
}

// GenerateKey generates a key using the registered generator for the specified version.
func (km *KeyManager) GenerateKey(version uint32) (string, error) {
	generator, exists := km.generatorList[version]
	if !exists {
		return "", fmt.Errorf("key generator not registered for version: %d", version)
	}

	key, err := generator.GenerateKey()
	return key, err
}

// ParseKey parses a key string by extracting its prefix to find the matching version parser.
func (km *KeyManager) ParseKey(keyStr string) (*ecdh.PublicKey, string, error) {
	// Extract prefix (characters before the first '-').
	before, _, ok := strings.Cut(keyStr, "-")
	if !ok {
		return nil, "", fmt.Errorf("invalid key format: missing prefix separator")
	}
	prefix := before

	version, exists := km.prefixToVersion[prefix]
	if !exists {
		return nil, "", fmt.Errorf("unsupported key prefix: %s", prefix)
	}

	parser, exists := km.ecdhParserList[version]
	if !exists {
		// Auto-initialize v1 parser if it's the known v1 key type but not yet registered.
		if version != v1keytype {
			return nil, "", fmt.Errorf("no parser registered for version: %d", version)
		}
		parser = NewKeyParserV1()
		km.ecdhParserList[version] = parser
	}

	return parser.ParseKey(keyStr)
}
