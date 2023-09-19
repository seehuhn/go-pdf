// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package cmap

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/type1"
)

func TestAppendEncoded(t *testing.T) {
	// Create a new utf8Encoder instance
	g2c := NewIdentityGIDToCID()
	e := NewCIDEncoderUTF8(g2c).(*utf8Encoder)

	// Append some encoded characters
	s := pdf.String{}
	for i := 0; i < 2; i++ {
		s = e.AppendEncoded(s, 1, []rune{'A'})
		s = e.AppendEncoded(s, 2, []rune{'B'})
		s = e.AppendEncoded(s, 3, []rune{'C'})
		s = e.AppendEncoded(s, 4, []rune{'A'})     // two glyphs with the same unicode
		s = e.AppendEncoded(s, 1, []rune{'a'})     // same glyph with a different unicode
		s = e.AppendEncoded(s, 5, []rune("Hello")) // a multi-rune glyph
	}

	// Check that the encoded characters are correct
	expected := pdf.String("ABC\uE000a\uE001ABC\uE000a\uE001")
	if !bytes.Equal(s, expected) {
		t.Errorf("AppendEncoded returned %v, expected %v", s, expected)
	}

	// Check that the cmap and tounicode maps are correct
	expectedMapping := map[charcode.CharCode]type1.CID{
		runeToCode('A'):      1,
		runeToCode('B'):      2,
		runeToCode('C'):      3,
		runeToCode('a'):      1,
		runeToCode('\uE000'): 4,
		runeToCode('\uE001'): 5,
	}
	expectedCMap := New(g2c.ROS(), utf8cs, expectedMapping)
	if !reflect.DeepEqual(e.CMap(), expectedCMap) {
		t.Errorf("CMap returned %v, expected %v", e.CMap(), expectedCMap)
	}
	expectedToUnicode := NewToUnicode(utf8cs, map[charcode.CharCode][]rune{
		runeToCode('A'):      {'A'},
		runeToCode('B'):      {'B'},
		runeToCode('C'):      {'C'},
		runeToCode('a'):      {'a'},
		runeToCode('\uE000'): {'A'},
		runeToCode('\uE001'): {'H', 'e', 'l', 'l', 'o'},
	})
	if d := cmp.Diff(e.ToUnicode(), expectedToUnicode); d != "" {
		t.Errorf("ToUnicode returned diff %v", d)
	}
}

func TestUTF8(t *testing.T) {
	// verify that the encoding equals standard UTF-8 encoding

	enc := NewCIDEncoderUTF8(NewIdentityGIDToCID())

	var buf pdf.String
	for r := rune(0); r <= 0x10_FFFF; r++ {
		if r >= 0xD800 && r <= 0xDFFF || r == 0xFFFD {
			continue
		}

		buf = enc.AppendEncoded(buf[:0], 0, []rune{r})
		buf2 := []byte(string(r))
		if !bytes.Equal(buf, buf2) {
			t.Fatalf("AppendEncoded(0x%04x) = %v, want %v", r, buf, buf2)
		}
	}
}
