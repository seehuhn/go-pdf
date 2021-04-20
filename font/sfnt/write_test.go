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

package sfnt

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/table"
)

func TestExport(t *testing.T) {
	tt, err := Open("../truetype/ttf/FreeSerif.ttf")
	if err != nil {
		t.Fatal(err)
	}

	out, err := os.Create("out.ttf")
	if err != nil {
		t.Fatal(err)
	}

	n, err := tt.Export(out, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = out.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = tt.Close()
	if err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat("out.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != n {
		t.Errorf("wrong size: %d != %d", fi.Size(), n)
	}
}

func TestWriteCmap(t *testing.T) {
	cmap1 := make(map[rune]font.GlyphID)
	for r := rune(32); r < 127; r++ {
		cmap1[r] = font.GlyphID(r - 30)
	}
	cmap2 := make(map[rune]font.GlyphID)
	for r := rune(32); r < 127; r++ {
		cmap2[r] = font.GlyphID((r-32)*7/6 + 2)
	}
	cmap3 := make(map[rune]font.GlyphID)
	for r := rune(32); r < 127; r++ {
		cmap3[r] = font.GlyphID((r-32)*20/19 + 2)
	}
	cmap4 := make(map[rune]font.GlyphID)
	for r := rune(32); r < 127; r++ {
		if r < 40 {
			cmap4[r] = font.GlyphID(2 + 2*r)
		} else if r < 50 {
			cmap4[r] = font.GlyphID(100 + r)
		} else {
			cmap4[r] = font.GlyphID(200 + 2*r)
		}
	}

	for _, cmap := range []map[rune]font.GlyphID{
		cmap1, cmap2, cmap3, cmap4,
	} {
		buf := &bytes.Buffer{}
		enc, err := writeSimpleCmap(buf, cmap)
		if err != nil {
			t.Error(err)
			continue
		}

		fmt.Printf("% x\n", buf.Bytes())
		fd := bytes.NewReader(buf.Bytes())
		cmapTable, err := table.ReadCmapTable(fd)
		if err != nil {
			t.Error(err)
			continue
		}

		i2rMap := make(map[int]rune)
		for r, idx := range cmap {
			cc := enc(idx)
			i2rMap[0xF000+int(cc[0])] = r
		}
		i2r := func(c int) rune { return i2rMap[c] }

		encRec := cmapTable.Find(3, 0)
		if encRec == nil {
			t.Error("no 3,0 subtable found")
			continue
		}
		out, err := encRec.LoadCmap(fd, i2r)
		if err != nil {
			t.Error(err)
			continue
		}

		if len(out) != len(cmap) {
			t.Errorf("wrong cmap length: %d != %d", len(out), len(cmap))
			continue
		}
		for r, idx := range out {
			if cmap[r] != idx {
				t.Errorf("wrong mapping: %04x -> %d != %d", r, idx, cmap[r])
				continue
			}
		}
	}
}
