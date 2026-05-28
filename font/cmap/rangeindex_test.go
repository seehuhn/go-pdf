// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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
	"testing"
)

func TestRangeIndex(t *testing.T) {
	cases := []struct {
		name        string
		first, last []byte
		code        []byte
		want        int
		ok          bool
	}{
		{
			name:  "single full byte",
			first: []byte{0x00}, last: []byte{0xFF},
			code: []byte{0x41}, want: 0x41, ok: true,
		},
		{
			name:  "two byte normal",
			first: []byte{0x00, 0x00}, last: []byte{0x00, 0xFF},
			code: []byte{0x00, 0x05}, want: 5, ok: true,
		},
		{
			// full-byte high position must contribute (byte-wrap regression):
			// the high byte 0x01 carries a full 0x100 multiplier, not 0.
			name:  "full-byte high position low code",
			first: []byte{0x00, 0x00}, last: []byte{0xFF, 0xFF},
			code: []byte{0x00, 0x41}, want: 0x41, ok: true,
		},
		{
			name:  "full-byte high position high code",
			first: []byte{0x00, 0x00}, last: []byte{0xFF, 0xFF},
			code: []byte{0x01, 0x41}, want: 0x141, ok: true,
		},
		{
			name:  "out of range",
			first: []byte{0x00, 0x10}, last: []byte{0x00, 0x20},
			code: []byte{0x00, 0x05}, want: 0, ok: false,
		},
		{
			name:  "length mismatch",
			first: []byte{0x00, 0x00}, last: []byte{0x00, 0xFF},
			code: []byte{0x05}, want: 0, ok: false,
		},
		{
			// 8-byte range whose index overflows int -> unmapped, no panic
			name:  "overflow",
			first: []byte{0, 0, 0, 0, 0, 0, 0, 0},
			last:  []byte{0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE},
			code:  []byte{0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE},
			want:  0, ok: false,
		},
		{
			// index exactly math.MaxInt32 is still representable
			name:  "max int32 boundary",
			first: []byte{0x00, 0x00, 0x00, 0x00}, last: []byte{0xFF, 0xFF, 0xFF, 0xFF},
			code: []byte{0x7F, 0xFF, 0xFF, 0xFF}, want: 0x7FFFFFFF, ok: true,
		},
		{
			// one past math.MaxInt32 -> unmapped, identical on 32- and 64-bit hosts
			name:  "past max int32",
			first: []byte{0x00, 0x00, 0x00, 0x00}, last: []byte{0xFF, 0xFF, 0xFF, 0xFF},
			code: []byte{0x80, 0x00, 0x00, 0x00}, want: 0, ok: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := rangeIndex(c.first, c.last, c.code)
			if got != c.want || ok != c.ok {
				t.Errorf("rangeIndex = %d, %v; want %d, %v", got, ok, c.want, c.ok)
			}
		})
	}
}

// a malformed ToUnicode CMap with an 8-byte range must not crash Lookup.
func TestToUnicodeLookupOverflow(t *testing.T) {
	code := []byte{0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE}

	t.Run("direct", func(t *testing.T) {
		tu := &ToUnicodeFile{Ranges: []ToUnicodeRange{{
			First:  []byte{0, 0, 0, 0, 0, 0, 0, 0},
			Last:   []byte{0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE},
			Values: []string{"A"},
		}}}
		if got, ok := tu.Lookup(code); ok || got != "" {
			t.Errorf("Lookup = %q, %v; want \"\", false", got, ok)
		}
	})

	t.Run("parsed", func(t *testing.T) {
		stream := []byte(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def
/CMapName /Evil def
/CMapType 2 def
1 begincodespacerange
<0000000000000000> <FEFEFEFEFEFEFEFE>
endcodespacerange
1 beginbfrange
<0000000000000000> <FEFEFEFEFEFEFEFE> <0041>
endbfrange
endcmap
CMapName currentdict /CMap defineresource pop
end
end
`)
		tu, err := readToUnicode(bytes.NewReader(stream))
		if err != nil {
			t.Fatal(err)
		}
		if got, ok := tu.Lookup(code); ok || got != "" {
			t.Errorf("Lookup = %q, %v; want \"\", false", got, ok)
		}
	})
}

// a 2-byte range spanning the full code space must distinguish codes that
// differ only in the high byte (byte-wrap regression).
func TestToUnicodeLookupByteWrap(t *testing.T) {
	tu := &ToUnicodeFile{Ranges: []ToUnicodeRange{{
		First:  []byte{0x00, 0x00},
		Last:   []byte{0xFF, 0xFF},
		Values: []string{"A"},
	}}}

	lo, ok := tu.Lookup([]byte{0x00, 0x41})
	if !ok {
		t.Fatal("Lookup(0x0041) failed")
	}
	hi, ok := tu.Lookup([]byte{0x01, 0x41})
	if !ok {
		t.Fatal("Lookup(0x0141) failed")
	}
	if lo == hi {
		t.Errorf("codes differing in high byte both map to %q", lo)
	}
}

func TestLookupCIDOverflow(t *testing.T) {
	code := []byte{0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE}

	t.Run("overflow falls through to notdef", func(t *testing.T) {
		f := &File{CIDRanges: []Range{{
			First: []byte{0, 0, 0, 0, 0, 0, 0, 0},
			Last:  []byte{0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE},
			Value: 100,
		}}}
		if got := f.LookupCID(code); got != 0 {
			t.Errorf("LookupCID = %d; want 0 (notdef)", got)
		}
	})

	t.Run("normal range", func(t *testing.T) {
		f := &File{CIDRanges: []Range{{
			First: []byte{0x00, 0x00},
			Last:  []byte{0x00, 0xFF},
			Value: 100,
		}}}
		if got := f.LookupCID([]byte{0x00, 0x05}); got != 105 {
			t.Errorf("LookupCID = %d; want 105", got)
		}
	})
}
