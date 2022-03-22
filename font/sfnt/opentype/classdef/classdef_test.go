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
)

func FuzzClassDef(f *testing.F) {
	f.Add([]byte{0, 1, 0, 1, 0, 3, 0, 1, 0, 2, 0, 1})
	f.Add([]byte{0, 1, 0, 0, 0, 0})
	f.Add([]byte{0, 2, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		buf := make([]byte, 256)
		info, err := Read(bytes.NewReader(data), buf)
		if err != nil {
			return
		}

		data2 := info.Encode()

		info2, err := Read(bytes.NewReader(data2), buf)
		if err != nil {
			t.Error(err)
		}

		if len(data2) > len(data) {
			fmt.Printf("A % x\n", data)
			fmt.Printf("B % x\n", data2)
			fmt.Println(info)
			fmt.Println(info2)
			t.Error("encode is longer than original")
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
