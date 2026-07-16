// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package interfaces

// IKeyGenerator defines the interface for generating self-signed invite keys.
type IKeyGenerator interface {
	GenerateKey() (string, error)
}

// IKeyParser defines the interface for parsing and verifying self-signed invite keys.
type IKeyParser[T any] interface {
	ParseKey(keyStr string) (T, string, error)
}
