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
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

func FuzzFdSelect(f *testing.F) {
	const nGlyphs = 100
	fds := []FdSelectFn{
		func(gid font.GlyphID) int { return 0 },
		func(gid font.GlyphID) int { return int(gid) / 60 },
		func(gid font.GlyphID) int { return int(gid) / 4 },
		func(gid font.GlyphID) int { return int(gid) },
		func(gid font.GlyphID) int { return int(gid/5) % 5 },
	}
	for _, fd := range fds {
		f.Add(fd.encode(nGlyphs))
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		p := parser.New(bytes.NewReader(in))
		err := p.SetRegion("FDSelect", 0, int64(len(in)))
		if err != nil {
			t.Fatal(err)
		}
		fdSelect, err := readFDSelect(p, nGlyphs, 10)
		if err != nil {
			return
		}

		in2 := fdSelect.encode(nGlyphs)
		if len(in2) > len(in) {
			t.Error("inefficient encoding")
		}

		p = parser.New(bytes.NewReader(in2))
		err = p.SetRegion("FDSelect", 0, int64(len(in2)))
		if err != nil {
			t.Fatal(err)
		}
		fdSelect2, err := readFDSelect(p, nGlyphs, 25)
		if err != nil {
			t.Fatal(err)
		}

		for i := font.GlyphID(0); i < nGlyphs; i++ {
			if fdSelect(i) != fdSelect2(i) {
				t.Errorf("%d: %d != %d", i, fdSelect(i), fdSelect2(i))
			}
		}
	})
}
