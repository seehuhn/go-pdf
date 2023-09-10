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
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/tounicode"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
)

func TestRoundTripGlyfComposite(t *testing.T) {
	ttf, err := gofont.TrueType(gofont.GoSmallcapsItalic)
	if err != nil {
		t.Fatal(err)
	}
	cs := charcode.UCS2
	ros := &type1.CIDSystemInfo{
		Registry:   "Test",
		Ordering:   "Merkwürdig",
		Supplement: 7,
	}

	fontCMap, err := ttf.CMapTable.GetBest()
	if err != nil {
		t.Fatal(err)
	}

	cmapData := make(map[charcode.CharCode]type1.CID)
	cmapData[charcode.CharCode('A')] = type1.CID(fontCMap.Lookup('A'))
	cmapData[charcode.CharCode('B')] = type1.CID(fontCMap.Lookup('B'))
	cmapData[charcode.CharCode('C')] = type1.CID(fontCMap.Lookup('C'))
	cmapInfo := cmap.New(ros, cs, cmapData)

	m := make(map[charcode.CharCode][]rune, 8)
	m[charcode.CharCode('A')] = []rune{'A'}
	m[charcode.CharCode('B')] = []rune{'B'}
	m[charcode.CharCode('C')] = []rune{'C'}
	toUnicode := tounicode.New(cs, m)

	maxCID := type1.CID(fontCMap.Lookup('C'))
	CID2GID := make([]glyph.ID, maxCID+1)
	for cid := type1.CID(0); cid <= maxCID; cid++ {
		CID2GID[cid] = glyph.ID(cid)
	}

	info1 := &EmbedInfoGlyfComposite{
		Font:       ttf,
		SubsetTag:  "ZZZZZZ",
		CMap:       cmapInfo,
		CIDToGID:   CID2GID,
		ToUnicode:  toUnicode,
		IsSmallCap: true,
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
	info2, err := ExtractGlyfComposite(rw, dicts)
	if err != nil {
		t.Fatal(err)
	}

	for _, info := range []*EmbedInfoGlyfComposite{info1, info2} {
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
