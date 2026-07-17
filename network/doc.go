// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

/*
Package network provides the core peer-to-peer networking functionalities for the GOTA ecosystem.
It handles low-level UDP transport, NAT punching, cryptographic handshakes, and packet multiplexing.

# Packet Architecture

GOTA uses four distinct packet categories to maintain high performance, crypto-agility, and robust security:

  - NatPunch Packets: Entirely plaintext packets sent to punch holes through firewalls and resolve reflexive
    addresses. They use simple magic bytes and ephemeral tokens to prevent replay or blind spoofing attacks.
  - Handshake Packets: Cryptographic negotiation messages used to establish safe, authenticated sessions.
    The initial handshake always begins using the baseline default protocol via peer identities.
  - Kernel Level Encrypted Packets: Standard data packets containing cleartext envelopes once decrypted.
    They carry a 32-bit Dispatch ID (DID) for routing payloads either to local Unix sockets or to internal services.
  - App-to-App Encrypted Packets: Opaque payload envelopes where the end applications manage their own
    secondary encryption schemes. The daemon handles these blindly, acting purely as a secure transport layer.

# Crypto-Agility, Protocol Manager & Post-Handshake Upgrades

To prevent downgrade attacks and keep cryptographic handshakes lightweight, GOTA splits session lifecycle
into two phases: Base Handshake and Post-Handshake Promotion. This is coordinated by the Protocol Manager:

 1. Base Handshake: Establish an immediate, lightweight authenticated tunnel using the baseline default
    cipher suite ("Noise_IKpsk2_25519_ChaChaPoly_SHA256").
 2. Capabilities Exchange (DID=0, ServiceID=0x0004): Underneath the safely encrypted baseline tunnel,
    the internal Protocol Manager service queries the peer's supported cryptographic capabilities.
 3. Post-Handshake Promotion: If both peers support a more secure protocol (such as hybrid Post-Quantum
    ML-KEM), the Protocol Manager orchestrates a secondary key exchange encapsulated entirely within
    the baseline encrypted channel.
 4. Hot Key Switch: Once the secondary handshake is verified, the Ingress and Egress engines are updated
    with the new cryptographic keys, and the temporary baseline keys are securely wiped from memory.
*/
package network
