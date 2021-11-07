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
	"fmt"
	"io"
	"testing"

	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/sfnt"
)

func TestReadCFF(t *testing.T) {
	tt, err := sfnt.Open("../opentype/otf/SourceSerif4-Regular.otf", nil)
	if err != nil {
		t.Fatal(err)
	}

	table := tt.Header.Find("CFF ")
	if table == nil {
		t.Fatal("no CFF table found")
	}
	length := int64(table.Length)
	fmt.Println("before:", length)
	tableFd := io.NewSectionReader(tt.Fd, int64(table.Offset), length)

	cff, err := ReadCFF(tableFd)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("topDict:")
	for _, op := range cff.topDict.keys() {
		args := cff.topDict[op]
		fmt.Printf("%s: %v\n", op, args)
	}

	err = tt.Close()
	if err != nil {
		t.Fatal(err)
	}

	blob, err := cff.EncodeCFF()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("after:", len(blob))

	// t.Fatal("not implemented")
}

func TestCharset(t *testing.T) {
	cases := []struct {
		blob   []byte
		nGlyph int
		first  sid
		last   sid
	}{
		{[]byte{0, 0, 1, 0, 3, 0, 15}, 4, 1, 15},
		{[]byte{1, 0, 2, 13}, 15, 2, 2 + 13},
		{[]byte{2, 0, 3, 2, 1}, 1 + 2*256 + 2, 3, 3 + 2*256 + 1},
	}

	for i, test := range cases {
		fmt.Println("test", i)
		r := bytes.NewReader(test.blob)
		p := parser.New(r)
		err := p.SetRegion("CFF", 0, int64(len(test.blob)))
		if err != nil {
			t.Fatal(err)
		}
		names, err := readCharset(p, test.nGlyph)
		if err != nil {
			t.Fatal(err)
		}

		if len(names) != test.nGlyph {
			t.Errorf("expected %d glyphs, got %d", test.nGlyph, len(names))
		}

		out, err := encodeCharset(names)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(out, test.blob) {
			t.Errorf("expected %v, got %v", test.blob, out)
		}
	}
}
