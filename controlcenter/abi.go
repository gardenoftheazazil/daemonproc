// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package controlcenter

import (
	"encoding/binary"
)

// ReadBytesParam parses a 2-byte length-prefixed byte slice from the front of payload:
// [ParamLength uint16][ParamData...].
// It returns the parameter bytes, the remaining unparsed payload bytes, and an execution status code.
func ReadBytesParam(payload []byte) (param, rest []byte, statusCode uint16) {
	if len(payload) < 2 {
		return nil, payload, ErrInvalidArgs
	}

	paramLen := int(binary.BigEndian.Uint16(payload[:2]))
	if len(payload) < 2+paramLen {
		return nil, payload, ErrInvalidArgs
	}

	return payload[2 : 2+paramLen], payload[2+paramLen:], Success
}
