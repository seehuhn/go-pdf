// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package simpleenc

import (
	"strings"
	"testing"

	"seehuhn.de/go/pdf/font/pdfenc"
)

// TestToUnicodeLongText checks that ToUnicode reports a glyph text which is
// too long for a ToUnicode CMap.
func TestToUnicodeLongText(t *testing.T) {
	for _, n := range []int{256, 257} {
		enc := NewSimple(500, "Test", &pdfenc.Standard)
		if _, err := enc.Encode(1, "", strings.Repeat("A", n), 500); err != nil {
			t.Fatal(err)
		}
		tu, err := enc.ToUnicode()
		if n <= 256 && (err != nil || tu == nil) {
			t.Errorf("n=%d: got %v, %v", n, tu, err)
		} else if n > 256 && err == nil {
			t.Errorf("n=%d: over-long text accepted", n)
		}
	}
}
