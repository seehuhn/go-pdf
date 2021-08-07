// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package parser

import (
	"testing"

	"seehuhn.de/go/pdf/font"
)

func TestGsub4_1(t *testing.T) {
	inGid := []font.GlyphID{
		0, 0, 1, 2, 3, 1, 2, 4, 1, 2, 0, 0, 2, 1, 0, 0,
	}
	expectedGid := []font.GlyphID{
		0, 0, 123, 124, 1, 2, 0, 0, 21, 0, 0,
	}

	in := make([]font.Glyph, len(inGid))
	for i, gid := range inGid {
		in[i].Gid = gid
	}
	expected := make([]font.Glyph, len(expectedGid))
	for i, gid := range expectedGid {
		expected[i].Gid = gid
	}

	gsub := &gsub4_1{
		cov: map[font.GlyphID]int{
			1: 0,
			2: 1,
		},
		repl: [][]ligature{
			{
				ligature{
					in:  []font.GlyphID{2, 2},
					out: 122,
				},
				ligature{
					in:  []font.GlyphID{2, 3},
					out: 123,
				},
				ligature{
					in:  []font.GlyphID{2, 4},
					out: 124,
				},
			},
			{
				ligature{
					in:  []font.GlyphID{1},
					out: 21,
				},
			},
		},
	}
	pos := 0
	for pos < len(in) {
		out, next := gsub.Apply(useAllGlyphs, in, pos)
		if next < 0 {
			if !isEqual(in, out) {
				t.Errorf("change without progress: %d vs %d",
					in, out)
			}
			pos++
		} else {
			if isEqual(in, out) {
				t.Errorf("progress %d -> %d without change: %d",
					pos, next, out)
			}
			pos = next
		}
		in = out
	}

	if !isEqual(in, expected) {
		t.Errorf("wrong output: %d vs %d", in, expected)
	}
}

func isEqual(in []font.Glyph, expected []font.Glyph) bool {
	equal := len(in) == len(expected)
	if equal {
		for i, glyph := range in {
			if expected[i].Gid != glyph.Gid {
				equal = false
				break
			}
		}
	}
	return equal
}
