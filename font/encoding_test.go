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
	"testing"
	"unicode"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/postscript/psenc"
	"seehuhn.de/go/postscript/type1/names"
)

func TestDescribeEncoding(t *testing.T) {
	funnyEncoding := make([]string, 256)
	for i := range funnyEncoding {
		funnyEncoding[i] = ".notdef"
	}
	funnyEncoding[0o001] = "funny"  // non-standard name
	funnyEncoding[0o101] = "A"      // common to all encodings
	funnyEncoding[0o102] = "C"      // clashes with all encodings
	funnyEncoding[0o103] = "B"      // clashes with all encodings
	funnyEncoding[0o104] = "D"      // common to all encodings
	funnyEncoding[0o142] = "Bsmall" // only in MacExpertEncoding
	funnyEncoding[0o201] = "A"      // double encode some characters
	funnyEncoding[0o202] = "B"      // double encode some characters
	funnyEncoding[0o203] = "C"      // double encode some characters
	funnyEncoding[0o204] = "D"      // double encode some characters
	funnyEncoding[0o214] = "OE"     // only in WinAnsiEncoding
	funnyEncoding[0o227] = "Scaron" // only in PdfDocEncoding
	funnyEncoding[0o341] = "AE"     // only in StandardEncoding
	funnyEncoding[0o347] = "Aacute" // only in MacRomanEncoding

	encodings := [][]string{
		pdfenc.StandardEncoding[:],
		pdfenc.MacRomanEncoding[:],
		pdfenc.MacExpertEncoding[:],
		pdfenc.WinAnsiEncoding[:],
		pdfenc.PDFDocEncoding[:],
		funnyEncoding,
	}

	for i, enc := range encodings {
		for j, builtin := range encodings {
			desc := DescribeEncodingType1(enc, builtin)
			if i == j {
				if desc != nil {
					t.Errorf("DescribeEncoding(%d, %d) = %v", i, j, desc)
				}
			}

			enc2, err := UndescribeEncoding(nil, desc, builtin)
			if err != nil {
				t.Error(err)
				continue
			}

			for c, name := range enc {
				if name == ".notdef" {
					continue
				}
				if enc2[c] != name {
					t.Errorf("UndescribeEncoding(%d, %d) = %v", i, j, enc2)
					break
				}
			}
		}
	}
}

// TestStandardEncoding verifies that the standard encodings in
// seehuhn.de/pdf/font/pdfenc and in seehuh.de/pdf/font are consistent.
func TestStandardEncodin(t *testing.T) {
	for code, name := range pdfenc.StandardEncoding {
		r1 := StandardEncoding.Decode(byte(code))
		var r2 rune
		if name == ".notdef" {
			r2 = unicode.ReplacementChar
		} else {
			rr2 := names.ToUnicode(string(name), false)
			if len(rr2) != 1 {
				t.Errorf("bad name: %s", name)
				continue
			}
			r2 = rr2[0]
		}
		if r1 != r2 {
			t.Errorf("StandardEncoding[%d] = %q != %q", code, r1, r2)
		}
	}
}

// TestWinAnsiEncoding verifies that the WinAnsiEncodings in
// seehuhn.de/pdf/font/pdfenc and in seehuh.de/pdf/font are consistent.
func TestWinAnsiEncoding(t *testing.T) {
	for code, name := range pdfenc.WinAnsiEncoding {
		r1 := WinAnsiEncoding.Decode(byte(code))
		var r2 rune
		if name == ".notdef" {
			r2 = unicode.ReplacementChar
		} else {
			rr2 := names.ToUnicode(string(name), false)
			if len(rr2) != 1 {
				t.Errorf("bad name: %s", name)
				continue
			}
			r2 = rr2[0]
		}
		if code == 0o240 && r1 == '\u00a0' {
			r1 = ' '
		}
		if code == 0o255 && r1 == '\u00ad' {
			r1 = '-'
		}
		if r1 != r2 {
			t.Errorf("WinAnsiEncoding[0o%03o] = %q != %q", code, r1, r2)
		}
	}
}

func TestMacRomanEncoding(t *testing.T) {
	for code, name := range pdfenc.MacRomanEncoding {
		r1 := MacRomanEncoding.Decode(byte(code))

		if name == ".notdef" && r1 == unicode.ReplacementChar {
			continue
		}
		rr := names.ToUnicode(string(pdfenc.MacRomanEncoding[code]), false)
		if len(rr) != 1 {
			t.Errorf("len(rr) != 1 for %d", code)
			continue
		}
		r2 := rr[0]

		if code == 0o312 && r1 == '\u00a0' {
			r1 = ' '
		}

		if r1 != r2 {
			t.Errorf("MacRomanEncoding[0o%03o] = %q != %q", code, r1, r2)
		}
	}
}

func TestMacExpertEncoding(t *testing.T) {
	t.Skip()
	for code, name := range pdfenc.MacExpertEncoding {
		r1 := MacExpertEncoding.Decode(byte(code))

		if name == ".notdef" && r1 == unicode.ReplacementChar {
			continue
		}
		rr := names.ToUnicode(string(pdfenc.MacExpertEncoding[code]), false)
		if len(rr) != 1 {
			t.Errorf("len(rr) != 1 for %d", code)
			continue
		}
		r2 := rr[0]

		if r1 != r2 {
			t.Errorf("MacExpertEncoding[0o%03o] = %q != %q", code, r1, r2)
		}
	}
}

// OldEncoding describes the correspondence between character codes and unicode
// characters for a simple PDF font.
// TODO(voss): remove
type OldEncoding interface {
	// Encode gives the character code for a given rune.  The second
	// return code indicates whether a mapping is available.
	Encode(r rune) (byte, bool)

	// Decode returns the rune corresponding to a given character code.  This
	// is the inverse of Encode for all runes where Encode returns true in the
	// second return value.  Decode() returns unicode.ReplacementChar for
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

func TestStandardEncoding2(t *testing.T) {
	std := StandardEncoding

	m1 := make(map[byte]rune)
	for i := 0; i < 256; i++ {
		c := byte(i)
		r := std.Decode(c)
		if r == unicode.ReplacementChar {
			continue
		}
		m1[c] = r
	}

	m2 := make(map[byte]rune)
	for name, code := range psenc.StandardEncodingRev {
		rr := names.ToUnicode(name, false)
		if len(rr) != 1 {
			t.Errorf("bad name: %s", name)
			continue
		}
		r := rr[0]
		c, ok := std.Encode(r)
		if !ok {
			t.Errorf("Encoding failed: %04x->xxx", r)
		}
		if c != code {
			t.Errorf("Encoding failed: %04x->%d", r, c)
		}
		m2[code] = r
	}

	if d := cmp.Diff(m1, m2); d != "" {
		t.Errorf("mismatch: %s", d)
	}
}

func TestStandardEncoding3(t *testing.T) {
	for c := 0; c < 256; c++ {
		psName := psenc.StandardEncoding[c]
		r := StandardEncoding.Decode(byte(c))
		if r == unicode.ReplacementChar && psName == ".notdef" {
			continue
		}
		if string(names.ToUnicode(psName, false)) != string([]rune{r}) {
			t.Errorf("bad mapping: %d %s %04x", c, psName, r)
		}
	}
}
