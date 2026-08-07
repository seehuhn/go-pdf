// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package simpleenc

import (
	"testing"

	"seehuhn.de/go/pdf/font/pdfenc"
)

func TestDefaultWidth(t *testing.T) {
	const defaultWidth = 999
	type entry struct {
		code  byte
		width float64
	}
	type testCase struct {
		name     string
		entries  []entry
		expected float64
	}
	cases := []testCase{
		{"empty", nil, defaultWidth},
		{"single left", []entry{{0, 600}}, defaultWidth},
		{"single middle", []entry{{255, 600}}, defaultWidth},
		{"single right", []entry{{255, 600}}, defaultWidth},
		{"three left", []entry{{0, 3}, {1, 3}, {2, 3}, {255, 1}}, 3},
		{"three right", []entry{{0, 1}, {253, 3}, {254, 3}, {255, 3}}, 3},
		{"single left, right", []entry{{0, 600}, {255, 600}}, 600},
		{"no savings", []entry{{0, 1}, {255, 2}}, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gd := NewSimple(defaultWidth, "", &pdfenc.WinAnsi)
			for _, e := range tc.entries {
				gd.info[e.code] = &codeInfo{Width: e.width}
			}

			if w := gd.DefaultWidth(); w != tc.expected {
				t.Errorf("got %g; expected %g", w, tc.expected)
			}
		})
	}
}

// The font name decides which glyph list a glyph name is looked up in:
// ZapfDingbats names the pointing hand "a12", and that name means nothing in
// any other font.  A font program taken out of a PDF file names the subset it
// is, so the tag must not reach the decision.
func TestNewSimpleIgnoresSubsetTag(t *testing.T) {
	const pointingHand = "\u261e"

	for _, name := range []string{"ZapfDingbats", "AAAAAA+ZapfDingbats"} {
		enc := NewSimple(0, name, &pdfenc.Standard)

		// The text is what the glyph name already implies, so no ToUnicode
		// entry is needed: that is only true if the dingbats glyph list was
		// used for both the code choice and the mapping.
		if _, err := enc.Encode(1, "a12", pointingHand, 100); err != nil {
			t.Fatal(err)
		}
		if tu := enc.ToUnicode(); tu != nil {
			t.Errorf("%s: %q needed a ToUnicode entry, so the glyph name did not imply it",
				name, pointingHand)
		}
	}
}
