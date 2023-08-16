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
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/postscript/type1"
)

func TestRoundTripCID(t *testing.T) {
	otf, err := gofont.OpenType(gofont.GoRegular)
	if err != nil {
		t.Fatal(err)
	}
	cs := charcode.NewCodeSpace([]charcode.Range{
		{Low: []byte{0x04}, High: []byte{0x07}},
		{Low: []byte{0x10, 0x12}, High: []byte{0x11, 0x13}},
	})
	ros := &type1.CIDSystemInfo{
		Registry:   "Test",
		Ordering:   "Sonderbar",
		Supplement: 13,
	}
	cmap := make(map[charcode.CharCode]type1.CID, 8)
	for code := charcode.CharCode(0); code < 8; code++ {
		cmap[code] = type1.CID(2*code + 1)
	}
	toUnicode := make(map[charcode.CharCode][]rune, 8)
	for code := charcode.CharCode(0); code < 8; code++ {
		toUnicode[code] = []rune{'X', '0' + rune(code)}
	}
	info := &EmbedInfoComposite{
		Font:       otf.AsCFF(),
		SubsetTag:  "ABCDEF",
		CS:         cs,
		ROS:        ros,
		CMap:       cmap,
		ToUnicode:  toUnicode,
		UnitsPerEm: otf.UnitsPerEm,
		Ascent:     otf.Ascent,
		Descent:    otf.Descent,
		CapHeight:  otf.CapHeight,
		IsSerif:    otf.IsSerif,
		IsScript:   otf.IsScript,
	}

	rw := pdf.NewData(pdf.V1_7)
	ref := rw.Alloc()
	err = info.Embed(rw, ref)
	if err != nil {
		t.Fatal(err)
	}

	info2, err := ExtractComposite(rw, ref)
	if err != nil {
		t.Fatal(err)
	}

	// The floating point numbers in the glyphs may be represented differently.
	// Let's hope the Glyphs are ok.
	info.Font.Glyphs = nil
	info2.Font.Glyphs = nil

	// Functions are difficult to compare.
	info.Font.FDSelect = nil
	info2.Font.FDSelect = nil

	if d := cmp.Diff(info, info2); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}
