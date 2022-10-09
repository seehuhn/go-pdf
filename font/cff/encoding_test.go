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
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/font/glyph"
	"seehuhn.de/go/pdf/font/parser"
)

func FuzzEncoding(f *testing.F) {
	ss := &cffStrings{}
	var cc []int32

	var glyphs []*Glyph
	for i := 0; i < 258; i++ {
		var name string
		if i == 0 {
			name = ".notdef"
		} else if i >= 'A' && i <= 'Z' {
			name = string([]rune{rune(i)})
		} else {
			name = fmt.Sprintf("%d", i)
		}
		glyphs = append(glyphs, &Glyph{Name: name})
		cc = append(cc, ss.lookup(name))
	}

	f.Fuzz(func(t *testing.T, data1 []byte) {
		p := parser.New("test", bytes.NewReader(data1))
		enc1, err := readEncoding(p, cc)
		if err != nil {
			return
		}

		var enc2 []glyph.ID
		var data2 []byte
		if isStandardEncoding(enc1, glyphs) {
			enc2 = StandardEncoding(glyphs)
		} else if isExpertEncoding(enc1, glyphs) {
			enc2 = expertEncoding(glyphs)
		} else {
			data2, err = encodeEncoding(enc1, cc)
			if err != nil {
				t.Fatal(err)
			}

			p = parser.New("test", bytes.NewReader(data2))
			enc2, err = readEncoding(p, cc)
			if err != nil {
				t.Fatal(err)
			}
		}

		if diff := cmp.Diff(enc1, enc2); diff != "" {
			t.Errorf("different (-old +new):\n%s", diff)
		}
	})
}
