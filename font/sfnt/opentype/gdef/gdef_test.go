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

package gdef

import (
	"bytes"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
)

func FuzzGdef(f *testing.F) {
	table := &Table{}
	f.Add(table.Encode())
	table.GlyphClass = classdef.Table{
		2:  GlyphClassBase,
		3:  GlyphClassBase,
		4:  GlyphClassBase,
		10: GlyphClassLigature,
	}
	f.Add(table.Encode())
	table.MarkAttachClass = classdef.Table{
		5: 1,
		6: 2,
		7: 1,
	}
	f.Add(table.Encode())
	table.MarkGlyphSets = []coverage.Set{
		{12: true, 13: true, 14: true},
		{10: true, 15: true, 16: true},
	}
	f.Add(table.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		table1, err := Read(bytes.NewReader(data))
		if err != nil {
			return
		}

		data2 := table1.Encode()

		table2, err := Read(bytes.NewReader(data2))
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(table1, table2) {
			t.Error("different")
		}
	})
}
