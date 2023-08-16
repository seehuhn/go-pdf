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

package cff

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/sfnt/glyph"
)

func TestRoundTrip(t *testing.T) {
	otf, err := gofont.OpenType(gofont.GoRegular)
	if err != nil {
		t.Fatal(err)
	}

	encoding := make([]glyph.ID, 256)
	encoding[65] = otf.CMap.Lookup('A')
	encoding[66] = otf.CMap.Lookup('C')

	toUnicode := map[charcode.CharCode][]rune{
		65: {'A'},
		66: {'C'},
	}

	info1 := &EmbedInfoSimple{
		Font:       otf.AsCFF(),
		SubsetTag:  "UVWXYZ",
		Encoding:   encoding,
		ToUnicode:  toUnicode,
		UnitsPerEm: otf.UnitsPerEm,
		Ascent:     otf.Ascent,
		Descent:    otf.Descent,
		CapHeight:  otf.CapHeight,
		IsSerif:    true, // Just for testing
		IsAllCap:   true, // Just for testing
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
	info2, err := ExtractSimple(rw, dicts)
	if err != nil {
		t.Fatal(err)
	}

	// Compare encodings:
	if len(info1.Encoding) != len(info2.Encoding) {
		t.Fatalf("len(info1.Encoding) != len(info2.Encoding): %d != %d", len(info1.Encoding), len(info2.Encoding))
	}
	for i, gid := range info1.Encoding {
		if gid != 0 && gid != info2.Encoding[i] {
			t.Errorf("info1.Encoding[%d] != info2.Encoding[%d]: %d != %d", i, i, gid, info2.Encoding[i])
		}
	}

	for _, info := range []*EmbedInfoSimple{info1, info2} {
		info.Encoding = nil // already compared above

		// TODO(voss): reenable this once https://github.com/google/go-cmp/issues/335 is resolved
		info.Font.Outlines = nil
	}

	if d := cmp.Diff(info1, info2); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}
