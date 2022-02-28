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
	"reflect"
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

		buf := data.encode()

		if count == 0 && len(buf) != 2 {
			t.Error("wrong length for empty INDEX")
		}

		r := bytes.NewReader(buf)
		p := parser.New(r)
		err := p.SetRegion("CFF", 0, int64(len(buf)))
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

func FuzzIndex(f *testing.F) {
	iSeed := cffIndex{}
	buf := iSeed.encode()
	f.Add(buf)
	for _, d := range [][]byte{{}, {0}, {0, 1, 2, 3}} {
		iSeed = append(iSeed, d)
		buf := iSeed.encode()
		f.Add(buf)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		p := parser.New(r)
		err := p.SetRegion("CFF", 0, int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		i1, err := readIndex(p)
		if err != nil {
			return
		}

		buf := i1.encode()
		if len(buf) > len(data) {
			t.Error("inefficient encoding")
		}

		r = bytes.NewReader(buf)
		p = parser.New(r)
		err = p.SetRegion("CFF", 0, int64(len(buf)))
		if err != nil {
			t.Fatal(err)
		}
		i2, err := readIndex(p)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(i1, i2) {
			t.Error("unequal")
		}
	})
}
