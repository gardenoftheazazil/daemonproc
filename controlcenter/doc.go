// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

// Package controlcenter implements the Application Binary Interface (ABI) layer for the GOTA core daemon.
//
// It acts as the central orchestrator (the "high mind") that intercept local system calls (syscalls)
// transmitted from client applications via the IPC layer (libgota). This package validates binary layouts,
// extracts execution contexts, authenticates calling processes using automated DID injection, prevents resource
// abuse, and marshals return values back to the originating transport layer.
//
// Request Frame Layout (Binary Wire Format):
// Every system call request received from the local application must match the following 32-bit aligned header:
//   - Packet Length (16 bits): Total length of the payload. The Most Significant Bit (MSB, 0x8000) represents
//     the Control Flag (must be 1 for control messages, 0 for pure network proxying).
//   - Command Type  (16 bits): The internal Opcode identifying the registered ABI function to execute.
//   - Parameter Body (Variable): Dynamic raw binary values formatted strictly in Network Byte Order (Big Endian).
//
// Response Frame Layout (Binary Wire Format):
// The evaluation output returned by the control center back to the local application follows a mirrored structure:
//   - Packet Length (16 bits): Total length of the return frame including the header boundary.
//   - Control Flag  (1 bit)  : Multiplexed within the length header (always 1 for daemon-originated management).
//   - Command Type  (16 bits): The matching tracking Opcode that specifies which request this response belongs to.
//   - Return Payload (Variable): Binary serialized execution results or platform-agnostic structures.
//
// ABI Subsystems and Functional Groups:
//
// 1. Connection Subsystem (Opcode Group 0x0100): Handles core peer routing management. Examples include:
//   - ConnectRemotePeer: Parses structured invite keys and binds asynchronous egress tunnels.
//   - DisconnectPeer: Safely tears down active P2P connections and triggers immediate memory zeroization.
//
// 2. Identity & Security Subsystem (Opcode Group 0x0200): Coordinates cryptographic state operations:
//   - GetInviteKey: Generates authenticated invite tokens scoped to specified quantum-safe profile versions.
//   - RenewIdentity: Regenerates local Curve25519 public/private identities and updates global discovery tables.
//
// 3. Mesh & Diagnostics Subsystem (Opcode Group 0x0300): Manages hop-by-hop mesh routing and telemetries.
package controlcenter
