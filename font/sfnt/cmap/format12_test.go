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

package cmap

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFormat12Samples(t *testing.T) {
	// TODO(voss): remove
	names, err := filepath.Glob("../../../demo/try-all-fonts/cmap/12-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) < 2 {
		t.Fatal("not enough samples")
	}
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		_, err = decodeFormat12(data)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func FuzzFormat12(f *testing.F) {
	f.Add(format12{
		{startCharCode: 10, endCharCode: 20, startGlyphID: 30},
		{startCharCode: 1000, endCharCode: 2000, startGlyphID: 41},
		{startCharCode: 2000, endCharCode: 3000, startGlyphID: 1},
	}.Encode(0))

	f.Fuzz(func(t *testing.T, data []byte) {
		c1, err := decodeFormat12(data)
		if err != nil {
			return
		}

		data2 := c1.Encode(0)
		if len(data2) > len(data) {
			t.Error("too long")
		}

		c2, err := decodeFormat12(data2)
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(c1, c2) {
			t.Error("not equal")
		}
	})
}

var _ Subtable = format12(nil)
