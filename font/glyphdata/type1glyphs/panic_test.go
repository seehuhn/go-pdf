// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package type1glyphs

import (
	"io"
	"testing"

	"seehuhn.de/go/pdf/font/glyphdata"
)

// TestFromStreamPanicIsError checks that a panic while extracting the font
// data is returned as an error instead of terminating the process.
func TestFromStreamPanicIsError(t *testing.T) {
	stream := &glyphdata.Stream{
		Type: glyphdata.Type1,
		WriteTo: func(io.Writer, *glyphdata.Lengths) error {
			panic("boom")
		},
	}
	if _, err := FromStream(stream); err == nil {
		t.Error("expected an error from a panicking stream")
	}
}
