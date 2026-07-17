// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package interfaces

// IEgress represents the interface for sending local application data to the P2P network.
type IEgress interface {
	// RouteToNetwork routes local application payloads to the network.
	// Egress is responsible for parsing routing headers (remote peer, remote DID),
	// encrypting the payload, and sending it to the correct destination.
	RouteToNetwork(srcDID DID, data []byte) error
}
