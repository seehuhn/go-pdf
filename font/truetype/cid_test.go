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

package truetype_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/internal/testfont"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
)

func TestRoundTripComposite(t *testing.T) {
	ttf := testfont.MakeGlyfFont()

	cs := charcode.CodeSpaceRange{
		{Low: []byte{0x04}, High: []byte{0x07}},
		{Low: []byte{0x10, 0x12}, High: []byte{0x11, 0x13}},
	}
	ros := &cid.SystemInfo{
		Registry:   "Test",
		Ordering:   "Sonderbar",
		Supplement: 13,
	}
	cmapData := make(map[charcode.CharCode]cid.CID, 8)
	for code := charcode.CharCode(0); code < 8; code++ {
		cmapData[code] = cid.CID(2*code + 1)
	}
	cmapInfo := cmap.New(ros, cs, cmapData)
	m := make(map[charcode.CharCode][]rune, 8)
	for code := charcode.CharCode(0); code < 8; code++ {
		m[code] = []rune{'X', '0' + rune(code)}
	}
	toUnicode := cmap.NewToUnicode(cs, m)

	info1 := &truetype.EmbedInfoComposite{
		Font:      ttf,
		SubsetTag: "AAAAAA",
		CMap:      cmapInfo,
		CID2GID:   []glyph.ID{0, 1, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6, 0, 7, 0, 8},
		ToUnicode: toUnicode,
		IsAllCap:  true, // just for testing
	}

	rw := pdf.NewData(pdf.V1_7)
	ref := rw.Alloc()
	err := info1.Embed(rw, ref)
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(rw, ref)
	if err != nil {
		t.Fatal(err)
	}
	info2, err := truetype.ExtractComposite(rw, dicts)
	if err != nil {
		t.Fatal(err)
	}

	for _, info := range []*truetype.EmbedInfoComposite{info1, info2} {
		info.Font.CMapTable = nil // "cmap" table is optional

		info.Font.FamilyName = ""        // "name" table is optional
		info.Font.Width = 0              // "OS/2" table is optional
		info.Font.Weight = 0             // "OS/2" table is optional
		info.Font.IsSerif = false        // "OS/2" table is optional
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
