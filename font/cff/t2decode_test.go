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
	"fmt"
	"reflect"
	"testing"
)

func TestRoll(t *testing.T) {
	in := []fixed16{1, 2, 3, 4, 5, 6, 7, 8}
	out := []fixed16{1, 2, 4, 5, 6, 3, 7, 8}

	roll(in[2:6], 3)
	for i, x := range in {
		if out[i] != x {
			t.Error(in, out)
			break
		}
	}
}

func FuzzT2Decode(f *testing.F) {
	f.Add(t2endchar.Bytes())
	f.Fuzz(func(t *testing.T, data1 []byte) {
		info := &decodeInfo{
			subr:         cffIndex{},
			gsubr:        cffIndex{},
			defaultWidth: 500,
			nominalWidth: 666,
		}
		g1, err := decodeCharString(info, data1)
		if err != nil {
			return
		}

		data2, err := g1.encodeCharString(info.defaultWidth, info.nominalWidth)
		if err != nil {
			t.Fatal(err)
		}

		g2, err := decodeCharString(info, data2)
		if err != nil {
			fmt.Printf("A % x\n", data1)
			fmt.Printf("A %s\n", g1)
			fmt.Printf("B % x\n", data2)
			fmt.Printf("B %s\n", g2)
			t.Fatal(err)
		}

		if !reflect.DeepEqual(g1, g2) {
			fmt.Printf("A % x\n", data1)
			fmt.Printf("A %s\n", g1)
			fmt.Printf("B % x\n", data2)
			fmt.Printf("B %s\n", g2)
			t.Error("different")
		}
	})
}
