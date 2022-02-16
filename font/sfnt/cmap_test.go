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

// TODO(voss): move this over to the new code

func TestMakeCMap(t *testing.T) {
	var mapping []font.CMapEntry
	check := make(map[rune]font.GlyphID)
	for i, c := range []int{32, 65, 66, 67, 68, 70, 71, 90, 92} {
		gid := font.GlyphID(i + 1)
		mapping = append(mapping, font.CMapEntry{
			CharCode: uint16(c),
			GID:      gid,
		})
		check[rune(c)] = gid
	}

	buf, err := makeCMap(mapping)
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

	for charCode, gid := range check {
		if cmap[charCode] != gid {
			t.Errorf("wrong mapping: cmap[%d] == %d != %d",
				charCode, cmap[charCode], gid)
		}
	}
	for charCode, gid := range cmap {
		if check[charCode] != gid {
			t.Errorf("wrong mapping: cmap[%d] == %d != %d",
				charCode, gid, check[charCode])
		}
	}
}

func TestFunnyEnd(t *testing.T) {
	mapping := []font.CMapEntry{
		{CharCode: 32, GID: 1},
		{CharCode: 44, GID: 2},
		{CharCode: 48, GID: 3},
		{CharCode: 49, GID: 4},
		{CharCode: 50, GID: 5},
		{CharCode: 74, GID: 6},
		{CharCode: 76, GID: 7},
		{CharCode: 85, GID: 8},
		{CharCode: 86, GID: 9},
		{CharCode: 99, GID: 10},
		{CharCode: 100, GID: 11},
		{CharCode: 101, GID: 12},
		{CharCode: 102, GID: 13},
		{CharCode: 104, GID: 14},
		{CharCode: 105, GID: 15},
		{CharCode: 110, GID: 16},
		{CharCode: 111, GID: 17},
		{CharCode: 114, GID: 18},
		{CharCode: 115, GID: 19},
		{CharCode: 116, GID: 20},
		{CharCode: 118, GID: 21},
		{CharCode: 121, GID: 22},
		{CharCode: 127, GID: 23},
		{CharCode: 128, GID: 24},
		{CharCode: 129, GID: 25},
		{CharCode: 130, GID: 26},
	}

	check := make(map[rune]font.GlyphID)
	for _, m := range mapping {
		check[rune(m.CharCode)] = m.GID
	}

	buf, err := makeCMap(mapping)
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
	fmt.Println(cmap)

	for charCode, gid := range check {
		if cmap[charCode] != gid {
			t.Errorf("wrong mapping: cmap[%d] == %d != %d",
				charCode, cmap[charCode], gid)
		}
	}
	for charCode, gid := range cmap {
		if check[charCode] != gid {
			t.Errorf("wrong mapping: cmap[%d] == %d != %d",
				charCode, gid, check[charCode])
		}
	}
}

func TestSplitSegments(t *testing.T) {
	cases := []struct {
		in  []font.CMapEntry
		out []int
	}{
		{ // single delta segment
			[]font.CMapEntry{{CharCode: 1, GID: 1}, {CharCode: 2, GID: 2}, {CharCode: 10, GID: 10}},
			[]int{0, 3},
		},
		{ // single GlyphIDArray segment
			[]font.CMapEntry{{CharCode: 1, GID: 1}, {CharCode: 2, GID: 3}, {CharCode: 3, GID: 5}},
			[]int{0, 3},
		},
		{ // single glyph
			[]font.CMapEntry{{CharCode: 1, GID: 1}},
			[]int{0, 1},
		},
		{ // a short GlyphIDArray segment is cheaper than two delta segments
			[]font.CMapEntry{{CharCode: 1, GID: 1}, {CharCode: 3, GID: 1}},
			[]int{0, 2},
		},
		{ // a long GlyphIDArray segment is more expensive than two delta segments
			[]font.CMapEntry{{CharCode: 1, GID: 1}, {CharCode: 5, GID: 1}},
			[]int{0, 1, 2},
		},
		{ // the example from the source code
			[]font.CMapEntry{{CharCode: 1, GID: 1}, {CharCode: 2, GID: 2}, {CharCode: 5, GID: 5},
				{CharCode: 6, GID: 10}, {CharCode: 7, GID: 11}, {CharCode: 8, GID: 6}},
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
