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
	"time"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/internal/many"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
)

func TestRoundTripCFFComposite(t *testing.T) {
	otf, err := many.OpenType(many.GoRegular)
	if err != nil {
		t.Fatal(err)
	}
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
	info1 := &EmbedInfoCFFComposite{
		Font:      otf,
		SubsetTag: "ABCDEF",
		CMap:      cmapInfo,
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
	info2, err := ExtractCFFComposite(rw, dicts)
	if err != nil {
		t.Fatal(err)
	}

	// normalize the fonts before comparing them
	for _, font := range []*sfnt.Font{info1.Font, info2.Font} {
		// LineGap is stored in the "hmtx" and "OS/2" tables.
		font.LineGap = 0

		// Width is stored in the "OS/2" table.
		font.Width = 0

		// IsRegular is stored in the "OS/2" table.
		font.IsRegular = false

		// CodePageRange is stored in the "OS/2" table.
		font.CodePageRange = 0

		// CreationTime and ModificationTime are stored in the "head" table.
		font.CreationTime = time.Time{}
		font.ModificationTime = time.Time{}

		// Description and License are stored in the "name" table.
		font.Description = ""
		font.License = ""

		// The floating point numbers in the glyphs may be represented differently.
		// Let's hope the Glyphs are ok.
		outlines := font.Outlines.(*cff.Outlines)
		outlines.Glyphs = nil

		// Functions are difficult to compare.
		outlines.FDSelect = nil
	}

	if d := cmp.Diff(info1, info2); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}

var _ font.Embedded = (*embeddedCFFComposite)(nil)
