// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package cff_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestCompositeCollectionCMap writes a CFF font with a predefined CMap of a
// registered character collection, whose CIDs are neither small nor
// contiguous, and reads it back.
func TestCompositeCollectionCMap(t *testing.T) {
	const text = "§ 5 ± 3 °C"

	f, err := cmap.Predefined("UniJIS-UCS2-H")
	if err != nil {
		t.Fatal(err)
	}
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	F, err := cff.NewComposite(makefont.OpenType(), &cff.OptionsComposite{CMap: f})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	var s pdf.String
	codec := F.Codec()
	for _, g := range F.Layout(nil, 12, text).Seq {
		c, ok := F.Encode(g.GID, g.Text)
		if !ok {
			t.Fatalf("cannot encode %q", g.Text)
		}
		s = codec.AppendCode(s, c)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := dictObj.(*dict.CIDFontType0)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}

	if d.CMap != f {
		t.Errorf("CMap is %q, want the predefined UniJIS-UCS2-H", d.CMap.Name)
	}
	if d.ROS.Registry != "Adobe" || d.ROS.Ordering != "Japan1" {
		t.Errorf("collection %v, want Adobe-Japan1", d.ROS)
	}
	if d.ToUnicode != nil {
		t.Error("ToUnicode written for a collection whose text is known")
	}

	// the widths are keyed on the collection's CIDs
	section := f.LookupCID([]byte{0x00, 0xA7})
	if _, ok := d.Width[section]; !ok {
		t.Errorf("no width for CID %d (§)", section)
	}

	cffFont, err := cffglyphs.FromStream(d.FontFile)
	if err != nil {
		t.Fatal(err)
	}
	if !cffFont.IsCIDKeyed() {
		t.Error("embedded font is not CID-keyed")
	}

	got := ""
	for c := range d.MakeFont().Codes(s) {
		got += c.Text
	}
	if got != text {
		t.Errorf("text reads back as %q, want %q", got, text)
	}
}
