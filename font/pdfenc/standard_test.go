// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pdfenc

import (
	"testing"

	"seehuhn.de/go/postscript/psenc"
	"seehuhn.de/go/postscript/type1/names"
)

// TestStandardEncoding tests that the PDF standard encoding is
// the same as the postscript standard encoding.
func TestStandardEncoding(t *testing.T) {
	for code, name := range Standard.Encoding {
		if name != psenc.StandardEncoding[code] {
			t.Errorf("StandardEncoding[%d] = %q != %q", code, name, psenc.StandardEncoding[code])
		}
	}
}

// TestStandardEncoding2 tests that all encoded entries in StandardEncoding
// correspond to a single unicode rune.
func TestStandardEncoding2(t *testing.T) {
	for _, name := range Standard.Encoding {
		if name == ".notdef" {
			continue
		}
		rr := names.ToUnicode(name, "")
		if len([]rune(rr)) != 1 {
			t.Errorf("StandardEncoding[%q] = %q, which is not a single unicode rune", name, rr)
		}
	}
}
