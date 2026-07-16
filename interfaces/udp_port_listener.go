// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package interfaces

import (
	"context"
	"net"
)

// IUdpPortListener is an interface that defines the methods for
// sending UDP packets to a specific IP address and port.
type IUdpPortListener interface {
	// StartListener starts listening on the specified UDP port.
	StartListener(port uint16, ctx context.Context) error

	// StopListener stops listening on the UDP port.
	StopListener() error

	// SendPacketTo sends a UDP packet to the specified IP address
	// and port with the given data.
	SendPacketTo(targetAddr *net.UDPAddr, data []byte) error

	// SubscribeToPacketReceived allows subscribing to incoming
	// UDP packets. The provided callback function will be called
	// whenever a packet is received. If the callback returns true,
	// the packet will be sent to the packetChannel; otherwise it
	// will be discarded.
	//
	// In the SUBSCRIBER (reader) goroutine, use a non-blocking
	// consumer loop like this:
	//
	//	go func(ctx context.Context) {
	//	    for {
	//	        select {
	//	        case packet := <-packetChannel:
	//	            // Process the received packet.
	//	        case <-ctx.Done():
	//	            return
	//	        }
	//	    }
	//	}(ctx)
	//
	// In the LISTENER (sender) implementation, use this
	// non-blocking write to prevent memory leaks if the
	// channel blocks:
	//
	//	select {
	//	case packetChannel <- data:
	//	    // Packet successfully pushed to the channel.
	//	default:
	//	    // Drop packet safely if the channel is full or blocked.
	//	}
	//
	// Every callback function will be called in a separate
	// goroutine, so it is safe to perform long-running operations
	// in the callback without blocking the listener. The function
	// returns a subscription ID that can be used to unsubscribe
	// from the packet received event.
	SubscribeToPacketReceived(
		callback func(sourceAddr *net.UDPAddr, data []byte) bool,
		packetChannel chan []byte,
	) (uint32, error)

	// UnsubscribeFromPacketReceived allows unsubscribing from the
	// packet received event using the provided subscription ID.
	UnsubscribeFromPacketReceived(subscriptionID uint32) error
}
