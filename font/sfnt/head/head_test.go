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

package head

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func TestHeadLength(t *testing.T) {
	info := &Info{}
	data, _ := info.Encode()
	if len(data) != headLength {
		t.Errorf("expected %d, got %d", headLength, len(data))
	}
}

func FuzzHead(f *testing.F) {
	info := &Info{}
	data, _ := info.Encode()
	f.Add(data)

	f.Fuzz(func(t *testing.T, d1 []byte) {
		i1, err := Read(bytes.NewReader(d1))
		if err != nil {
			return
		}

		d2, err := i1.Encode()
		if err != nil {
			t.Fatal(err)
		}

		i2, err := Read(bytes.NewReader(d2))
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(i1, i2) {
			fmt.Println(i1)
			fmt.Println(i2)
			t.Fatal("not equal")
		}
	})
}
