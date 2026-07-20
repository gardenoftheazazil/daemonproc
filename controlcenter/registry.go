// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package controlcenter

import (
	"errors"

	"github.com/gardenoftheazazil/daemonproc/invitekey"
)

// ErrNilDispatcher is returned when attempting to register handlers into a nil Dispatcher.
var ErrNilDispatcher = errors.New("dispatcher cannot be nil")

// RegisterDefaultHandlers registers all default daemon ABI system calls into the provided Dispatcher.
func RegisterDefaultHandlers(d *Dispatcher, km *invitekey.KeyManager) error {
	if d == nil {
		return ErrNilDispatcher
	}

	if km != nil {
		err := d.RegisterSysCall(OpcodeGetInviteKey, SyscallDescriptor{
			Opcode:  OpcodeGetInviteKey,
			Name:    "GetInviteKey",
			Handler: MakeGetInviteKeyHandler(km),
		})
		if err != nil {
			return err
		}
	}

	return nil
}
