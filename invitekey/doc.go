// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

// Package invitekey generate, parse and validate invite keys for peer-to-peer connections.
// Example key structure like that:
// gota[version number]-base58(ip:port|peeridentity cryptographic validator).digital signature for the key
// ip port info neccessary for NAT punching, peer identity is necessary for cryptographic handshake validation, signature is necessary to validate the authenticity of the key.
package invitekey
