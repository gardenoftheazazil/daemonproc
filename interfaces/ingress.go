// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package interfaces

// IIngress represents the interface for managing incoming connections and routing them to the appropriate handlers.
// Ingress is responsible for accepting incoming connections, performing any necessary authentication or validation.
// And then forwarding the connection to the appropriate service or handler based on the request details.
// Ingress resolve encrypted message through informatin in the provided handshaker_session information.
type IIngress interface {

	// Subscribe to port listener upon callback function and listen encrypted incoming data.
	Start(portListener IUdpPortListener)

	// Session information provide neccessary elements for:
	//  * decrypting incoming data
	//  * routing it to the appropriate handler
	//
	// Return session id for future reference and subscription to the session.
	AddSession(session HandshakeSession) (sessionID int)

	SubscribeToSession(sessionID string, callback func(data []byte) bool) (subscriptionID string, err error)

	UnsubscribeFromSession(sessionID string, subscriptionID string) error

	RemoveSession(sessionID string) error
}
