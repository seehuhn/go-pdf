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

package truetype_test

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt/glyph"
)

// embedWithCMap embeds a composite font written with the given CMap, sets
// text, and returns the font dictionary read back and the encoded text.
func embedWithCMap(t *testing.T, f *cmap.File, text string) (*dict.CIDFontType2, pdf.String) {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	F, err := truetype.NewComposite(makefont.TrueType(), &truetype.OptionsComposite{CMap: f})
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
	d, ok := dictObj.(*dict.CIDFontType2)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}
	return d, s
}

func TestCMapOptionIdentity(t *testing.T) {
	for _, c := range []struct {
		f     *cmap.File
		name  string
		wMode font.WritingMode
	}{
		{nil, "Identity-H", font.Horizontal},
		{mustPredefined(t, "Identity-V"), "Identity-V", font.Vertical},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, _ := embedWithCMap(t, c.f, "Hello")
			if !d.CMap.IsPredefined() || d.CMap.Name != c.name {
				t.Errorf("CMap is %q, want the predefined %s", d.CMap.Name, c.name)
			}
			if d.CMap.WMode != c.wMode {
				t.Errorf("writing mode %v, want %v", d.CMap.WMode, c.wMode)
			}
		})
	}
}

// TestCMapOptionUTF8 checks that a UTF-8 template embeds a CMap of its own
// and puts the text itself into the content stream.
func TestCMapOptionUTF8(t *testing.T) {
	const text = "Größe"
	d, s := embedWithCMap(t, cmap.UTF8H, text)
	if d.CMap.IsPredefined() {
		t.Errorf("CMap %q is predefined, want an embedded one", d.CMap.Name)
	}
	if !bytes.Equal(s, []byte(text)) {
		t.Errorf("encoded text is % x, want the UTF-8 bytes of %q", []byte(s), text)
	}
	if d.CMap.ROS == nil || d.ROS == nil || *d.CMap.ROS != *d.ROS {
		t.Errorf("CMap collection %v, font collection %v", d.CMap.ROS, d.ROS)
	}
}

func TestCMapOptionTemplateCodeSpace(t *testing.T) {
	f := &cmap.File{CodeSpaceRange: charcode.UCS2}
	if _, err := truetype.NewComposite(makefont.TrueType(), &truetype.OptionsComposite{CMap: f}); err == nil {
		t.Error("template with a UCS-2 code space was accepted")
	}
}

func mustPredefined(t *testing.T, name string) *cmap.File {
	t.Helper()
	f, err := cmap.Predefined(name)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// TestCMapOptionPrivateCollection writes a font with a hand-written CMap
// over a character collection the library knows nothing about.  The caller
// has to say which glyph each CID means.
func TestCMapOptionPrivateCollection(t *testing.T) {
	const text = "HELLO"

	info := makefont.TrueType()
	lookup, err := info.CMapTable.GetBest()
	if err != nil {
		t.Fatal(err)
	}

	// the CMap maps the codes 'A'..'Z' to the CIDs 100..125
	ros := &cid.SystemInfo{Registry: "Test", Ordering: "Private"}
	f := &cmap.File{
		Name:           "Test-Private-H",
		ROS:            ros,
		CodeSpaceRange: charcode.Simple,
		CIDRanges:      []cmap.Range{{First: []byte{'A'}, Last: []byte{'Z'}, Value: 100}},
	}
	table := make(map[glyph.ID]cid.CID)
	for r := 'A'; r <= 'Z'; r++ {
		table[lookup.Lookup(r)] = cid.CID(100 + r - 'A')
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	F, err := truetype.NewComposite(info, &truetype.OptionsComposite{
		CMap:     f,
		GIDToCID: cmap.NewGIDToCIDFromMap(ros, table),
	})
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

	// the codes are the letters themselves
	if !bytes.Equal(s, []byte(text)) {
		t.Errorf("encoded text is % x, want the bytes of %q", []byte(s), text)
	}

	x := pdf.NewExtractor(w)
	dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := dictObj.(*dict.CIDFontType2)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}
	if d.ROS == nil || d.ROS.Registry != "Test" || d.ROS.Ordering != "Private" {
		t.Errorf("collection %v, want Test-Private", d.ROS)
	}
	if d.CMap.IsPredefined() {
		t.Errorf("CMap %q is predefined, want an embedded one", d.CMap.Name)
	}

	// every code reaches the glyph the table names, with the text intact
	got := ""
	for c := range d.MakeFont().Codes(s) {
		got += c.Text
		if want := table[lookup.Lookup([]rune(c.Text)[0])]; c.CID != want {
			t.Errorf("%q decodes to CID %d, want %d", c.Text, c.CID, want)
		}
	}
	if got != text {
		t.Errorf("text reads back as %q, want %q", got, text)
	}
}
