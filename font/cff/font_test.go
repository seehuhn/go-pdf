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

package cff

import (
	"io"
	"testing"

	"seehuhn.de/go/pdf/font/sfnt"
)

func TestReadCFF(t *testing.T) {
	tt, err := sfnt.Open("../opentype/otf/SourceSerif4-Regular.otf", nil)
	if err != nil {
		t.Fatal(err)
	}

	table := tt.Header.Find("CFF ")
	if table == nil {
		t.Fatal("no CFF table found")
	}
	length := int64(table.Length)
	tableFd := io.NewSectionReader(tt.Fd, int64(table.Offset), length)

	err = readCFF(tableFd, length)
	if err != nil {
		t.Error(err)
	}

	err = tt.Close()
	if err != nil {
		t.Fatal(err)
	}

	t.Error("fish")
}
