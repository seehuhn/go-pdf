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

package kern

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/sfnt/glyph"
)

func FuzzKern(f *testing.F) {
	kern := Info{}
	f.Add(kern.Encode())
	kern[glyph.Pair{Left: 0, Right: 0}] = 0
	f.Add(kern.Encode())
	kern[glyph.Pair{Left: 1, Right: 2}] = -10
	kern[glyph.Pair{Left: 2, Right: 2}] = 10
	kern[glyph.Pair{Left: 3, Right: 2}] = 100
	f.Add(kern.Encode())

	f.Fuzz(func(t *testing.T, data1 []byte) {
		info1, err := Read(bytes.NewReader(data1))
		if err != nil {
			return
		}

		data2 := info1.Encode()
		info2, err := Read(bytes.NewReader(data2))
		if err != nil {
			t.Fatal(err)
		}

		if d := cmp.Diff(info1, info2); d != "" {
			t.Errorf("kern mismatch (-want +got):\n%s", d)
		}
	})
}
