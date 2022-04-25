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

package coverage

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf/font/parser"
)

func FuzzCoverageTable(f *testing.F) {
	f.Add([]byte{0, 1, 0, 0})
	f.Add([]byte{0, 1, 0, 3, 1, 0, 1, 1, 1, 2})
	f.Add([]byte{0, 2, 0, 0})
	f.Add([]byte{0, 2, 0, 1, 1, 0, 1, 2, 0, 0})
	f.Add([]byte{0, 2, 0, 2, 1, 0, 1, 2, 0, 0, 2, 0, 2, 5, 0, 3})
	f.Fuzz(func(t *testing.T, data1 []byte) {
		c1, err := Read(parser.New("coverage table test", bytes.NewReader(data1)), 0)
		if err != nil {
			return
		}

		data2 := c1.Encode()

		c2, err := Read(parser.New("coverage table test", bytes.NewReader(data2)), 0)
		if err != nil {
			t.Fatal(err)
		}

		if len(data2) > len(data1) {
			t.Error("inefficient encoding")
		}

		if !reflect.DeepEqual(c1, c2) {
			fmt.Printf("A % x\n", data1)
			fmt.Printf("B % x\n", data2)
			fmt.Println(c1)
			fmt.Println(c2)
			t.Fatal("different")
		}
	})
}
