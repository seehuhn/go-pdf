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

package classdef

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf/font/parser"
)

func FuzzClassDef(f *testing.F) {
	f.Add([]byte{0, 1, 0, 1, 0, 3, 0, 1, 0, 2, 0, 1})
	f.Add([]byte{0, 1, 0, 0, 0, 0})
	f.Add([]byte{0, 2, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		info, err := Read(parser.New("test", bytes.NewReader(data)), 0)
		if err != nil {
			return
		}

		data2 := info.Append(nil)

		info2, err := Read(parser.New("test", bytes.NewReader(data2)), 0)
		if err != nil {
			fmt.Printf("A % x\n", data)
			fmt.Printf("B % x\n", data2)
			fmt.Println(info)
			t.Fatal(err)
		}

		if len(data2) > len(data) {
			fmt.Printf("A % x\n", data)
			fmt.Printf("B % x\n", data2)
			fmt.Println(info)
			fmt.Println(info2)
			t.Error("encode is longer than original")
		}

		if len(data2) != info.AppendLen() {
			t.Errorf("wrong length, expected %d, got %d", info.AppendLen(), len(data2))
		}

		if !reflect.DeepEqual(info, info2) {
			fmt.Printf("A % x\n", data)
			fmt.Printf("B % x\n", data2)
			fmt.Println(info)
			fmt.Println(info2)
			t.Error("different")
		}
	})
}
