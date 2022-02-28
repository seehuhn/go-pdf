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
	"fmt"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf/font"
)

func FuzzFormat6(f *testing.F) {
	f.Add((&format6{
		FirstCode:    123,
		GlyphIDArray: []font.GlyphID{6, 4, 2},
	}).Encode(0))

	f.Fuzz(func(t *testing.T, data []byte) {
		c1, err := decodeFormat6(data)
		if err != nil {
			return
		}

		data2 := c1.Encode(0)

		c2, err := decodeFormat6(data2)
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(c1, c2) {
			fmt.Printf("A: % x\n", data)
			fmt.Printf("B: % x\n", data2)
			t.Error("not equal")
		}
	})
}

var _ Subtable = (*format6)(nil)
