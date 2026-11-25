// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package image

import (
	"bytes"
	"io"
	"testing"
)

// TestDCTEncodePanicIsError checks that a panic in WriteData is returned as
// an error instead of terminating the process.
func TestDCTEncodePanicIsError(t *testing.T) {
	s := &DCTSource{
		Width:  2,
		Height: 2,
		Colors: 1,
		WriteData: func(io.Writer) error {
			panic("boom")
		},
	}
	var buf bytes.Buffer
	if err := s.encode(&buf); err == nil {
		t.Error("expected an error from a panicking WriteData")
	}
}
