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

package sfnt

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goregular"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
)

func TestPostscriptName(t *testing.T) {
	info := &Info{
		FamilyName: `A(n)d[r]o{m}e/d<a> N%ebula`,
		Weight:     font.WeightBold,
		IsItalic:   true,
	}
	psName := info.PostscriptName()
	if psName != "AndromedaNebula-BoldItalic" {
		t.Errorf("wrong postscript name: %q", psName)
	}

	var rr []rune
	for i := 0; i < 255; i++ {
		rr = append(rr, rune(i))
	}
	info.FamilyName = string(rr)
	psName = info.PostscriptName()
	if len(psName) != 127-33-10+len("-BoldItalic") {
		t.Errorf("wrong postscript name: %q", psName)
	}
}

func FuzzFont(f *testing.F) {
	f.Add(goregular.TTF)
	f.Add(gobolditalic.TTF)

	f.Fuzz(func(t *testing.T, data []byte) {
		font1, err := Read(bytes.NewReader(data))
		if err != nil {
			return
		}

		buf := &bytes.Buffer{}
		_, err = font1.Write(buf)
		if err != nil {
			t.Fatal(err)
		}

		font2, err := Read(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatal(err)
		}

		opt := cmp.Comparer(func(fn1, fn2 cff.FdSelectFn) bool {
			for gid := 0; gid < font1.NumGlyphs(); gid++ {
				if fn1(font.GlyphID(gid)) != fn2(font.GlyphID(gid)) {
					return false
				}
			}
			return true
		})
		if diff := cmp.Diff(font1, font2, opt); diff != "" {
			t.Errorf("different (-old +new):\n%s", diff)
		}
	})
}
