// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package truetype_test

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/internal/cidtest"
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
		{cmap.IdentityH, "Identity-H", font.Horizontal},
		{cmap.IdentityV, "Identity-V", font.Vertical},
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

// TestCompositeRequiresCMap checks that a composite font cannot be made
// without a CMap.
func TestCompositeRequiresCMap(t *testing.T) {
	if _, err := truetype.NewComposite(makefont.TrueType(), nil); err == nil {
		t.Error("nil options accepted")
	}
	if _, err := truetype.NewComposite(makefont.TrueType(), &truetype.OptionsComposite{}); err == nil {
		t.Error("nil CMap accepted")
	}
}

// TestCMapOptionNotdef checks that the font as written and the font read
// back agree on every field of [font.Code], for a hand-written CMap with
// notdef mappings to CIDs other than 0.
func TestCMapOptionNotdef(t *testing.T) {
	for _, wMode := range []font.WritingMode{font.Horizontal, font.Vertical} {
		t.Run(wMode.String(), func(t *testing.T) {
			info := makefont.TrueType()
			f, gidToCID := cidtest.NotdefCMap(t, info, wMode)
			F, err := truetype.NewComposite(info, &truetype.OptionsComposite{
				CMap:     f,
				GIDToCID: gidToCID,
			})
			if err != nil {
				t.Fatal(err)
			}
			cidtest.CheckNotdef(t, F)
		})
	}
}

// TestCompositeNotdefGlyph checks that, with a CMap whose collection implies a
// glyph for CID 0, CID 0 is still glyph 0 of the font.
func TestCompositeNotdefGlyph(t *testing.T) {
	info := makefont.TrueType()
	f, err := cmap.Predefined("Adobe-Japan1-7")
	if err != nil {
		t.Fatal(err)
	}
	F, err := truetype.NewComposite(info, &truetype.OptionsComposite{CMap: f})
	if err != nil {
		t.Fatal(err)
	}
	cidtest.CheckNotdefGlyph(t, F, info, "Hello")
}

// TestCompositeBadGIDToCID checks that a GIDToCID which does not map CID 0
// and glyph 0 to each other is rejected.
func TestCompositeBadGIDToCID(t *testing.T) {
	info := makefont.TrueType()
	f, gidToCID := cidtest.NotdefCMap(t, info, font.Horizontal)
	if _, err := truetype.NewComposite(info, &truetype.OptionsComposite{CMap: f, GIDToCID: gidToCID}); err != nil {
		t.Fatal(err)
	}

	for _, g2c := range []map[glyph.ID]cid.CID{{5: 0}, {0: 3}} {
		bad := cmap.NewGIDToCIDFromMap(gidToCID.ROS(), g2c)
		_, err := truetype.NewComposite(info, &truetype.OptionsComposite{CMap: f, GIDToCID: bad})
		if err == nil {
			t.Errorf("table %v accepted", g2c)
		}
	}
}
