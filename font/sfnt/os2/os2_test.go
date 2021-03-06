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

package os2

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func FuzzOS2(f *testing.F) {
	f.Fuzz(func(t *testing.T, in []byte) {
		i1, err := Read(bytes.NewReader(in))
		if err != nil {
			return
		}

		buf := i1.Encode(nil)
		i2, err := Read(bytes.NewReader(buf))
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(i1, i2) {
			fmt.Printf("%#v\n", i1)
			fmt.Printf("%#v\n", i2)
			t.Fatal("not equal")
		}
	})
}
