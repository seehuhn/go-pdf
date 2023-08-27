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

package opentype

import (
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/tounicode"
	"seehuhn.de/go/sfnt/glyph"
)

func TestRoundTripCFFSimple(t *testing.T) {
	otf, err := gofont.OpenType(gofont.GoItalic)
	if err != nil {
		t.Fatal(err)
	}

	cmap, err := otf.CMapTable.GetBest()
	if err != nil {
		t.Fatal(err)
	}

	encoding := make([]glyph.ID, 256)
	encoding[65] = cmap.Lookup('A')
	encoding[66] = cmap.Lookup('C')

	m := map[charcode.CharCode][]rune{
		65: {'A'},
		66: {'C'},
	}
	toUnicode := tounicode.FromMapping(charcode.Simple, m)

	info1 := &EmbedInfoCFFSimple{
		Font:      otf,
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
	info2, err := ExtractCFFSimple(rw, dicts)
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
	// Compare ascent, descent, capHeight in PDF units, since they come from
	// the PDF font descriptor:
	if math.Round(info1.Font.Ascent.AsFloat(q)) != math.Round(info2.Font.Ascent.AsFloat(q)) {
		t.Errorf("info1.Font.Ascent != info2.Font.Ascent: %f != %f", info1.Font.Ascent.AsFloat(q), info2.Font.Ascent.AsFloat(q))
	}
	if math.Round(info1.Font.Descent.AsFloat(q)) != math.Round(info2.Font.Descent.AsFloat(q)) {
		t.Errorf("info1.Font.Descent != info2.Font.Descent: %f != %f", info1.Font.Descent.AsFloat(q), info2.Font.Descent.AsFloat(q))
	}
	if math.Round(info1.Font.CapHeight.AsFloat(q)) != math.Round(info2.Font.CapHeight.AsFloat(q)) {
		t.Errorf("info1.Font.CapHeight != info2.Font.CapHeight: %f != %f", info1.Font.CapHeight.AsFloat(q), info2.Font.CapHeight.AsFloat(q))
	}

	for _, info := range []*EmbedInfoCFFSimple{info1, info2} {
		info.Encoding = nil // already compared above

		info.Font.Ascent = 0    // already compared above
		info.Font.Descent = 0   // already compared above
		info.Font.CapHeight = 0 // already compared above

		info.Font.Width = 0                      // "OS/2" table is optional
		info.Font.IsRegular = false              // "OS/2" table is optional
		info.Font.CodePageRange = 0              // "OS/2" table is optional
		info.Font.CreationTime = time.Time{}     // "head" table is optional
		info.Font.ModificationTime = time.Time{} // "head" table is optional
		info.Font.Description = ""               // "name" table is optional
		info.Font.Trademark = ""                 // "name" table is optional
		info.Font.License = ""                   // "name" table is optional
		info.Font.LicenseURL = ""                // "name" table is optional
		info.Font.PermUse = 0                    // "OS/2" table is optional
		info.Font.LineGap = 0                    // "OS/2" and "hmtx" tables are optional
		info.Font.XHeight = 0                    // "OS/2" table is optional

		// TODO(voss): make this match, and ignore font.CMap instead
		info.Font.CMapTable = nil

		// TODO(voss): reenable this once https://github.com/google/go-cmp/issues/335 is resolved
		info.Font.Outlines = nil
	}

	if d := cmp.Diff(info1, info2); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}
