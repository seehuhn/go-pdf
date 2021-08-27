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
	subset := make([]font.GlyphID, 100)
	for i, c := range []int{32, 65, 66, 67, 68, 70, 71, 90} {
		subset[c] = font.GlyphID(i + 1)
	}

	buf, err := makeSimpleCmap(subset)
	if err != nil {
		t.Fatal(err)
	}

	r := bytes.NewReader(buf)
	cmapTable, err := table.ReadCmapTable(r)
	if err != nil {
		t.Fatal(err)
	}

	if cmapTable.Header.NumTables != 1 {
		t.Errorf("wrong number of tables: %d != 1", cmapTable.Header.NumTables)
	}
	enc := cmapTable.Find(3, 0)
	cmap, err := enc.LoadCmap(r, func(i int) rune { return rune(i) })
	if err != nil {
		t.Fatal(err)
	}

	for r := rune(0); r < 256; r++ {
		var expected font.GlyphID
		if int(r) < len(subset) {
			expected = subset[r]
		}
		if cmap[r] != expected {
			t.Errorf("wrong mapping: cmap[%d] == %d != %d",
				r, cmap[r], expected)
		}
	}
}
