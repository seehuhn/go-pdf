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

package gtab

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf/sfnt/parser"
)

func FuzzFeatureList(f *testing.F) {
	info := FeatureListInfo{}
	info = append(info, &Feature{Tag: "test"})
	f.Add(info.encode())
	info = append(info, &Feature{Tag: "kern", Lookups: []LookupIndex{0, 1, 2, 3}})
	f.Add(info.encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		p := parser.New(bytes.NewReader(data))
		info, err := readFeatureList(p, 0)
		if err != nil {
			return
		}

		data2 := info.encode()

		// if len(data2) > len(data) {
		// 	fmt.Printf("A % x\n", data)
		// 	fmt.Printf("B % x\n", data2)
		// 	t.Errorf("encode: %d > %d", len(data2), len(data))
		// }

		p = parser.New(bytes.NewReader(data2))
		info2, err := readFeatureList(p, 0)
		if err != nil {
			fmt.Printf("A % x\n", data)
			fmt.Printf("B % x\n", data2)
			fmt.Println(info)
			t.Fatal(err)
		}

		if !reflect.DeepEqual(info, info2) {
			// fmt.Printf("A % x\n", data)
			// fmt.Printf("B % x\n", data2)

			if len(info) != len(info2) {
				t.Fatal("different lengths")
			}
			fmt.Println("length:", len(info))
			for i, f1 := range info {
				f2 := info2[i]

				if f1.Tag != f2.Tag {
					t.Fatalf("info[%d].tag: %q != %q", i, f1.Tag, f2.Tag)
				}
			}
			t.Error("different")
		}
	})
}
