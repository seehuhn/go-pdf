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

package type1

import "testing"

func TestDecodeGlyphName(t *testing.T) {
	cases := []struct {
		glyph    string
		dingbats bool
		res      []rune
	}{
		{"Lcommaaccent", false, []rune{0x013B}},
		{"uni20AC0308", false, []rune{0x20AC, 0x0308}},
		{"u1040C", false, []rune{0x1040C}},
		{"uniD801DC0C", false, []rune{}},
		{"uni20ac", false, []rune{}},
		{"Lcommaaccent_uni20AC0308_u1040C.alternate",
			false, []rune{0x013B, 0x20AC, 0x0308, 0x1040C}},
		{"uni013B", false, []rune{0x013B}},
		{"u013B", false, []rune{0x013B}},
		{"foo", false, []rune{}},
		{".notdef", false, []rune{}},
		{"Ogoneksmall", false, []rune{0xF6FB}},
		{"a7", true, []rune{0x271E}},
	}
	for i, test := range cases {
		out := decodeGlyphName(test.glyph, test.dingbats)
		equal := len(out) == len(test.res)
		if equal {
			for j, c := range out {
				if test.res[j] != c {
					equal = false
					break
				}
			}
		}
		if !equal {
			t.Errorf("%d: expected %q but got %q",
				i, string(test.res), string(out))
		}
	}
}

func TestGlyphMap(t *testing.T) {
	cases := []struct {
		file, glyph string
		ok          bool
		res         rune
	}{
		{"zapfdingbats", "a100", true, 0x275E},
		{"zapfdingbats", "a128", true, 0x2468},
		{"zapfdingbats", "a9", true, 0x2720},
		{"zapfdingbats", "finger", false, 0},
		{"glyphlist", "A", true, 'A'},
		{"glyphlist", "Izhitsadblgravecyrillic", true, 0x0476},
		{"glyphlist", "zukatakana", true, 0x30BA},
		{"glyphlist", "END", false, 0},
	}

	for i, test := range cases {
		res, ok := glyph.lookup(test.file, test.glyph)
		if ok != test.ok || res != test.res {
			t.Errorf("%d: expected %t/%c but got %t/%c",
				i, test.ok, test.res, ok, res)
		}
	}
}
