// Copyright (c) 2026 Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.

package controlcenter_test

import (
	"bytes"
	"testing"

	"github.com/gardenoftheazazil/daemonproc/controlcenter"
)

func TestReadBytesParam(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		payload    []byte
		wantParam  []byte
		wantRest   []byte
		wantStatus uint16
	}{
		{
			name:       "nil payload",
			payload:    nil,
			wantParam:  nil,
			wantRest:   nil,
			wantStatus: controlcenter.ErrInvalidArgs,
		},
		{
			name:       "empty payload",
			payload:    []byte{},
			wantParam:  nil,
			wantRest:   []byte{},
			wantStatus: controlcenter.ErrInvalidArgs,
		},
		{
			name:       "short payload (1 byte)",
			payload:    []byte{0x00},
			wantParam:  nil,
			wantRest:   []byte{0x00},
			wantStatus: controlcenter.ErrInvalidArgs,
		},
		{
			name:       "length mismatch - declared 5 bytes but 2 provided",
			payload:    []byte{0x00, 0x05, 'a', 'b'},
			wantParam:  nil,
			wantRest:   []byte{0x00, 0x05, 'a', 'b'},
			wantStatus: controlcenter.ErrInvalidArgs,
		},
		{
			name:       "exact match - 0 bytes declared",
			payload:    []byte{0x00, 0x00},
			wantParam:  []byte{},
			wantRest:   []byte{},
			wantStatus: controlcenter.Success,
		},
		{
			name:       "exact match - 4 bytes declared and provided",
			payload:    []byte{0x00, 0x04, 't', 'e', 's', 't'},
			wantParam:  []byte("test"),
			wantRest:   []byte{},
			wantStatus: controlcenter.Success,
		},
		{
			name:       "chained parameters - extra bytes in rest",
			payload:    []byte{0x00, 0x04, 't', 'e', 's', 't', 0x00, 0x02, 'o', 'k'},
			wantParam:  []byte("test"),
			wantRest:   []byte{0x00, 0x02, 'o', 'k'},
			wantStatus: controlcenter.Success,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			param, rest, status := controlcenter.ReadBytesParam(tt.payload)
			if status != tt.wantStatus {
				t.Errorf("ReadBytesParam status = %d, wantStatus %d", status, tt.wantStatus)
			}
			if !bytes.Equal(param, tt.wantParam) {
				t.Errorf("ReadBytesParam param = %q, wantParam %q", param, tt.wantParam)
			}
			if !bytes.Equal(rest, tt.wantRest) {
				t.Errorf("ReadBytesParam rest = %q, wantRest %q", rest, tt.wantRest)
			}
		})
	}
}
