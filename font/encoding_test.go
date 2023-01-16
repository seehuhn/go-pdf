// seehuhn.de/go/pdf - a library for reading and writing PDF files
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

package font

import (
	"fmt"
	"testing"
	"unicode"

	"seehuhn.de/go/sfnt/type1/names"
)

// OldEncoding describes the correspondence between character codes and unicode
// characters for a simple PDF font.
type OldEncoding interface {
	// Encode gives the character code for a given rune.  The second
	// return code indicates whether a mapping is available.
	Encode(r rune) (byte, bool)

	// Decode returns the rune corresponding to a given character code.  This
	// is the inverse of Encode for all runes where Encode returns true in the
	// second return value.  DecodeX() returns unicode.ReplacementChar for
	// undefined code points.
	Decode(c byte) rune
}

func TestBuiltinEncodings(t *testing.T) {
	encodings := []OldEncoding{
		StandardEncoding,
		WinAnsiEncoding,
		MacRomanEncoding,
		MacExpertEncoding,
	}
	for i, enc := range encodings {
		r := enc.Decode(0)
		if r != unicode.ReplacementChar {
			t.Error("wrong mapping for character code 0")
		}
		_, ok := enc.Encode(unicode.ReplacementChar)
		if ok {
			t.Error("wrong mapping for unicode.ReplacementChar")
		}

		for j := 0; j < 256; j++ {
			c := byte(j)

			r := enc.Decode(c)
			if r == unicode.ReplacementChar {
				continue
			}
			c2, ok := enc.Encode(r)
			if !ok {
				t.Errorf("Encoding failed: %d %d->%04x->xxx", i, c, r)
			} else if c2 != c {
				t.Errorf("Encoding failed: %d %d->%04x->%d", i, c, r, c2)
			}
		}

		for r := rune(0); r < 65536; r++ {
			c, ok := enc.Encode(r)
			if !ok {
				continue
			}
			r2 := enc.Decode(c)
			if r2 == unicode.ReplacementChar {
				t.Errorf("Decoding failed: %d %04x->%d->xxx", i, r, c)
			} else if r2 != r {
				t.Errorf("Decoding failed: %d %04x->%d->%04x", i, r, c, r2)
			}
		}
	}
}

func TestXXX(t *testing.T) {
	for i := 0; i < 256; i++ {
		c := byte(i)
		r := MacExpertEncoding.Decode(c)
		if r == unicode.ReplacementChar {
			continue
		}
		name := names.FromUnicode(r)
		fmt.Printf("  %d: %q\n", c, name)
	}
}
