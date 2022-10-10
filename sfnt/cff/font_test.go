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

package cff

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/sfnt/glyph"
)

func FuzzFont(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		cff1, err := Read(bytes.NewReader(data))
		if err != nil {
			return
		}

		buf := &bytes.Buffer{}
		err = cff1.Encode(buf)
		if err != nil {
			fmt.Println(cff1)
			t.Fatal(err)
		}

		cff2, err := Read(bytes.NewReader(buf.Bytes()))
		if err != nil {
			return
		}

		cmpFdSelectFn := cmp.Comparer(func(fn1, fn2 FdSelectFn) bool {
			for gid := 0; gid < len(cff1.Glyphs); gid++ {
				if fn1(glyph.ID(gid)) != fn2(glyph.ID(gid)) {
					return false
				}
			}
			return true
		})
		cmpFloat := cmp.Comparer(func(x, y float64) bool {
			return math.Abs(x-y) < 5e-7
		})
		if diff := cmp.Diff(cff1, cff2, cmpFdSelectFn, cmpFloat); diff != "" {
			t.Errorf("different (-old +new):\n%s", diff)
		}
	})
}
