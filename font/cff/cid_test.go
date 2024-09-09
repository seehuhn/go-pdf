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
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/internal/debug/tempfile"
	"seehuhn.de/go/pdf/internal/makefont"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt/glyph"
)

func TestRoundTripComposite(t *testing.T) {
	otf := makefont.OpenType()
	cs := charcode.CodeSpaceRange{
		{Low: []byte{0x04}, High: []byte{0x07}},
		{Low: []byte{0x10, 0x12}, High: []byte{0x11, 0x13}},
	}
	ros := &cmap.CIDSystemInfo{
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
	info1 := &FontDictComposite{
		Font:       otf.AsCFF(),
		SubsetTag:  "ABCDEF",
		CMap:       cmapInfo,
		ToUnicode:  toUnicode,
		UnitsPerEm: otf.UnitsPerEm,
		Ascent:     otf.Ascent,
		Descent:    otf.Descent,
		CapHeight:  otf.CapHeight,
		IsSerif:    otf.IsSerif,
		IsScript:   otf.IsScript,
	}

	rw, _ := tempfile.NewTempWriter(pdf.V1_7, nil)

	ref := rw.Alloc()
	err := info1.Embed(rw, ref)
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(rw, ref)
	if err != nil {
		t.Fatal(err)
	}
	info2, err := ExtractComposite(rw, dicts)
	if err != nil {
		t.Fatal(err)
	}

	// The floating point numbers in the glyphs may be represented differently.
	// Let's hope the Glyphs are ok.
	for i := range info1.Font.Glyphs {
		info1.Font.Glyphs[i].Cmds = nil
	}
	for i := range info2.Font.Glyphs {
		info2.Font.Glyphs[i].Cmds = nil
	}

	// Functions are difficult to compare.
	for i := range info1.Font.Glyphs {
		gid := glyph.ID(i)
		if info1.Font.FDSelect(gid) != info2.Font.FDSelect(gid) {
			t.Errorf("FDSelect mismatch for glyph %d: %d != %d",
				i, info1.Font.FDSelect(gid), info2.Font.FDSelect(gid))
		}
	}
	info1.Font.FDSelect = nil
	info2.Font.FDSelect = nil

	if d := cmp.Diff(info1, info2); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}
