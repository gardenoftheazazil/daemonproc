// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

// Package stun provides functionality for discovering the public IP address and port of a client behind a NAT (Network Address Translation)
// using the STUN (Session Traversal Utilities for NAT) protocol.
//
// To activate, set stun as true in the config file and set stun server address in the stun_server_address field.
// The stun server address should be in the format "host:port" (e.g., "stun.l.google.com:19302").
//
// When these settings are activated, the STUN service runs automatically and keeps the STUN discovery process active in the background.
// It will update the public address and port of the client as it changes over time.
package stun
