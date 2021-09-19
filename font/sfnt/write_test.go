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
	"os"
	"testing"

	"seehuhn.de/go/pdf/font"
)

func TestExport(t *testing.T) {
	tt, err := Open("../truetype/ttf/SourceSerif4-Regular.ttf", nil)
	if err != nil {
		t.Fatal(err)
	}

	out, err := os.Create("out.ttf")
	if err != nil {
		t.Fatal(err)
	}

	subset := make([]font.GlyphID, 100)
	for i, c := range []int{32, 65, 66, 67, 68, 70, 71, 90} {
		subset[c] = font.GlyphID(i + 1)
	}
	var mapping []CMapEntry
	for cid, gid := range subset {
		if gid != 0 {
			mapping = append(mapping, CMapEntry{
				CID: uint16(cid),
				GID: gid,
			})
		}
	}
	opt := &ExportOptions{
		Include: map[string]bool{
			// The list of tables to include is from PDF 32000-1:2008, table 126.
			"glyf": true, // rewrite
			"head": true, // update CheckSumAdjustment, Modified and indexToLocFormat
			"hhea": true, // update various fields, including numberOfHMetrics (TODO)
			"hmtx": true, // rewrite
			"loca": true, // rewrite
			"maxp": true, // update numGlyphs
			"cvt ": true, // copy
			"fpgm": true, // copy
			"prep": true, // copy
		},
		Mapping: mapping,
	}

	n, err := tt.Export(out, opt)
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
