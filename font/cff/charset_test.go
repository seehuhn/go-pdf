// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package cff

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf/font/parser"
)

func TestCharsetDecode(t *testing.T) {
	cases := []struct {
		blob   []byte
		nGlyph int
		first  sid
		last   sid
	}{
		{[]byte{0, 0, 1, 0, 3, 0, 15}, 4, 1, 15},
		{[]byte{1, 0, 2, 13}, 15, 2, 2 + 13},
		{[]byte{2, 0, 3, 2, 1}, 1 + 2*256 + 2, 3, 3 + 2*256 + 1},
	}

	for _, test := range cases {
		p := parser.New(bytes.NewReader(test.blob))
		err := p.SetRegion("CFF", 0, int64(len(test.blob)))
		if err != nil {
			t.Fatal(err)
		}
		names, err := readCharset(p, test.nGlyph)
		if err != nil {
			t.Fatal(err)
		}

		if len(names) != test.nGlyph {
			t.Errorf("expected %d glyphs, got %d", test.nGlyph, len(names))
		}

		out, err := encodeCharset(names)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(out, test.blob) {
			t.Errorf("expected %v, got %v", test.blob, out)
		}
	}
}

func TestCharsetRoundtrip(t *testing.T) {
	n1 := make([]int32, 100)
	for i := range n1 {
		n1[i] = int32(2 * i)
	}

	n2 := make([]int32, 400)
	for i := range n2 {
		sid := int32(i)
		if i >= 300 && i <= 301 {
			sid = 1000 + 2*int32(i)
		} else if i > 300 {
			sid += 10
		}
		n2[i] = sid
	}

	n3 := make([]int32, 1200)
	for i := range n3 {
		sid := int32(i)
		if i == 600 {
			sid = 2000
		} else if i > 600 {
			sid += 10
		}
		n3[i] = sid
	}

	for i, names := range [][]int32{n1, n2, n3} {
		data, err := encodeCharset(names)
		if err != nil {
			t.Error(err)
			continue
		}
		if data[0] != byte(i) {
			t.Errorf("expected format %d, got %d", i, data[0])
		}

		p := parser.New(bytes.NewReader(data))
		err = p.SetRegion("CFF", 0, int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}

		out, err := readCharset(p, len(names))
		if err != nil {
			t.Fatal(err)
		}

		if len(out) != len(names) {
			t.Errorf("expected %d glyphs, got %d", len(names), len(out))
		}
		for i, sid := range names {
			if out[i] != sid {
				t.Errorf("expected %d, got %d", sid, out[i])
			}
		}
	}
}

func TestISOAdobe(t *testing.T) {
	// Appendix C of the ISO/IEC 14496-12:2015 specification has the names
	// in order or consecutive SID values.  This differs from other
	// "ISO-Adobe" character sets found on the web.
	ss := &cffStrings{}
	for i, name := range isoAdobeCharset {
		sid := ss.lookup(name)
		if int(sid) != i {
			t.Errorf("%q: expected %d, got %d", name, i, sid)
		}
	}

	// I can't think of an easy way to check that the other two character
	// sets are correct.  We restrict ourselves to check the spelling
	// by verifying that all names are in the list of default strings.
	for _, name := range expertCharset {
		sid := ss.lookup(name)
		if sid >= nStdString {
			t.Errorf("misspelled %q", name)
		}
	}
	for _, name := range expertSubsetCharset {
		sid := ss.lookup(name)
		if sid >= nStdString {
			t.Errorf("misspelled %q", name)
		}
	}
}

func FuzzCharset(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte, nGlyphs int) {
		r := bytes.NewReader(data)
		p := parser.New(r)
		err := p.SetRegion("CFF", 0, int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		names1, err := readCharset(p, nGlyphs)
		if err != nil {
			return
		}

		buf, err := encodeCharset(names1)
		if err != nil {
			t.Fatal(err)
		} else if len(buf) > len(data) {
			t.Error("inefficient encoding")
		}

		r = bytes.NewReader(buf)
		p = parser.New(r)
		err = p.SetRegion("CFF", 0, int64(len(buf)))
		if err != nil {
			t.Fatal(err)
		}
		names2, err := readCharset(p, nGlyphs)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(names1, names2) {
			fmt.Println(names1)
			fmt.Println(names2)
			t.Error("unequal")
		}
	})
}
