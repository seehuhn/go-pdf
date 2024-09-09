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

package opentype_test

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/internal/debug/tempfile"
	"seehuhn.de/go/pdf/internal/makefont"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
)

func TestRoundTripGlyfSimple(t *testing.T) {
	otf := makefont.TrueType()

	cmapInfo, err := otf.CMapTable.GetBest()
	if err != nil {
		t.Fatal(err)
	}

	encoding := make([]glyph.ID, 256)
	encoding[65] = cmapInfo.Lookup('A')
	encoding[66] = cmapInfo.Lookup('C')

	m := map[charcode.CharCode][]rune{
		65: {'A'},
		66: {'C'},
	}
	toUnicode := cmap.NewToUnicode(charcode.Simple, m)

	info1 := &opentype.FontDictGlyfSimple{
		Font:      otf,
		SubsetTag: "ABCXYZ",
		Encoding:  encoding,
		ToUnicode: toUnicode,
		IsAllCap:  true, // just for testing
	}

	rw, _ := tempfile.NewTempWriter(pdf.V1_7, nil)
	ref := rw.Alloc()
	err = info1.Embed(rw, ref)
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(rw, ref)
	if err != nil {
		t.Fatal(err)
	}
	info2, err := opentype.ExtractGlyfSimple(rw, dicts)
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

	q := 1000 / float64(info1.Font.UnitsPerEm)
	// Compare capHeight in PDF units, since this comes from the PDF font
	// descriptor:
	if math.Round(info1.Font.CapHeight.AsFloat(q)) != math.Round(info2.Font.CapHeight.AsFloat(q)) {
		t.Errorf("info1.Font.CapHeight != info2.Font.CapHeight: %f != %f", info1.Font.CapHeight.AsFloat(q), info2.Font.CapHeight.AsFloat(q))
	}

	for _, info := range []*opentype.FontDictGlyfSimple{info1, info2} {
		info.Encoding = nil       // already compared above
		info.Font.CMapTable = nil // already tested when comparing the encodings
		info.Font.Gdef = nil      // not included in PDF
		info.Font.Gsub = nil      // not included in PDF
		info.Font.Gpos = nil      // not included in PDF

		info.Font.CapHeight = 0 // already compared above

		info.Font.FamilyName = ""        // "name" table is optional
		info.Font.Width = 0              // "OS/2" table is optional
		info.Font.Weight = 0             // "OS/2" table is optional
		info.Font.IsRegular = false      // "OS/2" table is optional
		info.Font.CodePageRange = 0      // "OS/2" table is optional
		info.Font.Description = ""       // "name" table is optional
		info.Font.Copyright = ""         // "name" table is optional
		info.Font.Trademark = ""         // "name" table is optional
		info.Font.License = ""           // "name" table is optional
		info.Font.LicenseURL = ""        // "name" table is optional
		info.Font.XHeight = 0            // "OS/2" table is optional
		info.Font.UnderlinePosition = 0  // "post" table is optional
		info.Font.UnderlineThickness = 0 // "post" table is optional

		info.Font.Outlines.(*glyf.Outlines).Names = nil // "post" table is optional
	}

	if d := cmp.Diff(info1, info2); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}
