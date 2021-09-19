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
	cid2gid := make([]font.GlyphID, 100)
	for i, c := range []int{32, 65, 66, 67, 68, 70, 71, 90} {
		cid2gid[c] = font.GlyphID(i + 1)
	}

	buf, err := MakeCMap(cid2gid)
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

	for r := rune(0); r < 256; r++ {
		var expected font.GlyphID
		if int(r) < len(cid2gid) {
			expected = cid2gid[r]
		}
		if cmap[r] != expected {
			t.Errorf("wrong mapping: cmap[%d] == %d != %d",
				r, cmap[r], expected)
		}
	}
}

func TestMakeCMap2(t *testing.T) {
	var mm []CMapEntry
	check := make(map[rune]font.GlyphID)
	for i, c := range []int{32, 65, 66, 67, 68, 70, 71, 90, 92} {
		gid := font.GlyphID(i + 1)
		mm = append(mm, CMapEntry{
			CID: uint16(c),
			GID: gid,
		})
		check[rune(c)] = gid
	}

	buf, err := MakeCMap2(mm)
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
		in  []CMapEntry
		out []int
	}{
		{ // single delta segment
			[]CMapEntry{{1, 1}, {2, 2}, {10, 10}},
			[]int{0, 3},
		},
		{ // single GlyphIDArray segment
			[]CMapEntry{{1, 1}, {2, 3}, {3, 5}},
			[]int{0, 3},
		},
		{ // single glyph
			[]CMapEntry{{1, 1}},
			[]int{0, 1},
		},
		{ // a short GlyphIDArray segment is cheaper than two delta segments
			[]CMapEntry{{1, 1}, {3, 1}},
			[]int{0, 2},
		},
		{ // a long GlyphIDArray segment is more expensive than two delta segments
			[]CMapEntry{{1, 1}, {5, 1}},
			[]int{0, 1, 2},
		},
		{ // the example from the source code
			[]CMapEntry{{1, 1}, {2, 2}, {5, 5}, {6, 10}, {7, 11}, {8, 6}},
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
