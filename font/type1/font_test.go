// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package type1

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/gofont"
)

func TestRoundTrip(t *testing.T) {
	t1, err := gofont.Type1(gofont.GoRegular)
	if err != nil {
		t.Fatal(err)
	}

	encoding := make([]string, 256)
	for i := range encoding {
		encoding[i] = ".notdef"
	}
	encoding[65] = "A"
	encoding[66] = "B"

	m := map[charcode.CharCode][]rune{
		65: {'A'},
		66: {'B'},
	}
	toUnicode := cmap.NewToUnicode(charcode.Simple, m)

	info1 := &EmbedInfo{
		Font:      t1,
		SubsetTag: "UVWXYZ",
		Encoding:  encoding,
		ToUnicode: toUnicode,
	}

	rw := pdf.NewData(pdf.V1_7)
	ref := rw.Alloc()
	err = info1.Embed(rw, ref)
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(rw, ref)
	if err != nil {
		t.Fatal(err)
	}
	info2, err := Extract(rw, dicts)
	if err != nil {
		t.Fatal(err)
	}

	// Compare encodings:
	if len(info1.Encoding) != len(info2.Encoding) {
		t.Fatalf("len(info1.Encoding) != len(info2.Encoding): %d != %d", len(info1.Encoding), len(info2.Encoding))
	}
	for i := range info1.Encoding {
		if info1.Encoding[i] != ".notdef" && info1.Encoding[i] != info2.Encoding[i] {
			t.Fatalf("info1.Encoding[%d] != info2.Encoding[%d]: %q != %q", i, i, info1.Encoding[i], info2.Encoding[i])
		}
	}

	for _, info := range []*EmbedInfo{info1, info2} {
		info.Encoding = nil       // already compared above
		info.Font.XHeight = 0     // optional entry in FontDescriptor
		info.Font.GlyphInfo = nil // the bounding boxes sometimes differ
	}

	cmpFloat := cmp.Comparer(func(x, y float64) bool {
		return math.Abs(x-y) < 1/65536.
	})
	if d := cmp.Diff(info1, info2, cmpFloat); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}

func TestDefaultFontRoundTrip(t *testing.T) {
	t1, err := TimesItalic.PSFont()
	if err != nil {
		t.Fatal(err)
	}

	encoding := make([]string, 256)
	for i := range encoding {
		encoding[i] = ".notdef"
	}
	encoding[65] = "A"
	encoding[66] = "C"

	m := map[charcode.CharCode][]rune{
		65: {'A'},
		66: {'C'},
	}
	toUnicode := cmap.NewToUnicode(charcode.Simple, m)

	info1 := &EmbedInfo{
		Font:      t1,
		Encoding:  encoding,
		ToUnicode: toUnicode,
	}

	rw := pdf.NewData(pdf.V1_7)
	ref := rw.Alloc()
	err = info1.Embed(rw, ref)
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(rw, ref)
	if err != nil {
		t.Fatal(err)
	}
	info2, err := Extract(rw, dicts)
	if err != nil {
		t.Fatal(err)
	}

	// Compare encodings:
	if len(info1.Encoding) != len(info2.Encoding) {
		t.Fatalf("len(info1.Encoding) != len(info2.Encoding): %d != %d", len(info1.Encoding), len(info2.Encoding))
	}
	for i := range info1.Encoding {
		if info1.Encoding[i] != ".notdef" && info1.Encoding[i] != info2.Encoding[i] {
			t.Fatalf("info1.Encoding[%d] != info2.Encoding[%d]: %q != %q", i, i, info1.Encoding[i], info2.Encoding[i])
		}
	}

	for _, info := range []*EmbedInfo{info1, info2} {
		info.Encoding = nil       // already compared above
		info.Font.XHeight = 0     // optional entry in FontDescriptor
		info.Font.GlyphInfo = nil // the bounding boxes sometimes differ
	}

	cmpFloat := cmp.Comparer(func(x, y float64) bool {
		return math.Abs(x-y) < 1/65536.
	})
	if d := cmp.Diff(info1, info2, cmpFloat); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}

var _ font.Embedded = (*embedded)(nil)
