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

package glyf

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/go-test/deep"
)

func FuzzGlyf(f *testing.F) {
	info := &Info{
		Glyphs: []*Glyph{
			{tail: nil},
			{tail: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
			{tail: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}},
		},
	}
	buf := &bytes.Buffer{}
	locaData, locaFormat, err := info.Encode(buf)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes(), locaData, locaFormat)

	info = &Info{
		Glyphs: []*Glyph{
			{tail: bytes.Repeat([]byte{1, 2}, 32769)},
		},
	}
	buf.Reset()
	locaData, locaFormat, err = info.Encode(buf)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes(), locaData, locaFormat)

	f.Fuzz(func(t *testing.T, glyfData, locaData []byte, locaFormat int16) {
		info, err := Decode(glyfData, locaData, locaFormat)
		if err != nil {
			return
		}

		buf := &bytes.Buffer{}
		locaData2, locaFormat2, err := info.Encode(buf)
		if err != nil {
			t.Fatal(err)
		}
		glyfData2 := buf.Bytes()

		info2, err := Decode(glyfData2, locaData2, locaFormat2)
		if err != nil {
			t.Fatal(err)
		}

		different := false
		for _, diff := range deep.Equal(info, info2) {
			fmt.Println(diff)
			different = true
		}
		if different {
			fmt.Println("Ag", glyfData)
			fmt.Println("Al", locaData)
			fmt.Println("Bg", glyfData2)
			fmt.Println("Bl", locaData2)
			fmt.Println(info)
			fmt.Println(info2)
			t.Error("not equal")
		}
	})
}
