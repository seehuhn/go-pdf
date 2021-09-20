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

package sfnt

import (
	"bytes"
	"fmt"
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/table"
)

func TestMakeCMap(t *testing.T) {
	var mm []font.CMapEntry
	check := make(map[rune]font.GlyphID)
	for i, c := range []int{32, 65, 66, 67, 68, 70, 71, 90, 92} {
		gid := font.GlyphID(i + 1)
		mm = append(mm, font.CMapEntry{
			CID: uint16(c),
			GID: gid,
		})
		check[rune(c)] = gid
	}

	buf, err := MakeCMap(mm)
	if err != nil {
		t.Fatal(err)
	}

	r := bytes.NewReader(buf)
	cmapTable, err := table.ReadCMapTable(r)
	if err != nil {
		t.Fatal(err)
	}
	if cmapTable.Header.NumTables != 1 {
		t.Errorf("wrong number of tables: %d != 1", cmapTable.Header.NumTables)
	}
	enc := cmapTable.Find(1, 0)
	cmap, err := enc.LoadCMap(r, func(i int) rune { return rune(i) })
	if err != nil {
		t.Fatal(err)
	}

	for cid, gid := range check {
		if cmap[cid] != gid {
			t.Errorf("wrong mapping: cmap[%d] == %d != %d",
				cid, cmap[cid], gid)
		}
	}
}

func TestSplitSegments(t *testing.T) {
	cases := []struct {
		in  []font.CMapEntry
		out []int
	}{
		{ // single delta segment
			[]font.CMapEntry{{CID: 1, GID: 1}, {CID: 2, GID: 2}, {CID: 10, GID: 10}},
			[]int{0, 3},
		},
		{ // single GlyphIDArray segment
			[]font.CMapEntry{{CID: 1, GID: 1}, {CID: 2, GID: 3}, {CID: 3, GID: 5}},
			[]int{0, 3},
		},
		{ // single glyph
			[]font.CMapEntry{{CID: 1, GID: 1}},
			[]int{0, 1},
		},
		{ // a short GlyphIDArray segment is cheaper than two delta segments
			[]font.CMapEntry{{CID: 1, GID: 1}, {CID: 3, GID: 1}},
			[]int{0, 2},
		},
		{ // a long GlyphIDArray segment is more expensive than two delta segments
			[]font.CMapEntry{{CID: 1, GID: 1}, {CID: 5, GID: 1}},
			[]int{0, 1, 2},
		},
		{ // the example from the source code
			[]font.CMapEntry{{CID: 1, GID: 1}, {CID: 2, GID: 2}, {CID: 5, GID: 5},
				{CID: 6, GID: 10}, {CID: 7, GID: 11}, {CID: 8, GID: 6}},
			[]int{0, 3, 6},
		},
	}

	for i, test := range cases {
		ss := findSegments(test.in)
		a := fmt.Sprint(test.out)
		b := fmt.Sprint(ss)
		if a != b {
			t.Errorf("%d: expected %s, got %s", i, a, b)
		}
	}
}
