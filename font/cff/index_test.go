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
	"bytes"
	"testing"

	"seehuhn.de/go/pdf/font/parser"
)

func TestIndex(t *testing.T) {
	blob := make([]byte, 1+127)
	for i := range blob {
		blob[i] = byte(i + 1)
	}

	for _, count := range []int{0, 2, 3, 517} {
		data := make(cffIndex, count)
		for i := 0; i < count; i++ {
			d := i % 2
			data[i] = blob[d : d+127]
		}

		buf, err := data.encode()
		if err != nil {
			t.Error(err)
			continue
		}

		if count == 0 && len(buf) != 2 {
			t.Error("wrong length for empty INDEX")
		}

		r := bytes.NewReader(buf)
		p := parser.New(r)
		err = p.SetRegion("CFF", 0, int64(len(buf)))
		if err != nil {
			t.Fatal(err)
		}

		out, err := readIndex(p)
		if err != nil {
			t.Error(err)
			continue
		}
		if len(out) != len(data) {
			t.Errorf("wrong length")
			continue
		}
		for i, blob := range out {
			if !bytes.Equal(blob, data[i]) {
				t.Errorf("wrong data")
				continue
			}
		}
	}
}
