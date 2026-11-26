// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"fmt"
	"io"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	georect "seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt"
	sglyph "seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestFontExtent(t *testing.T) {
	enc := func(byte) (string, bool) { return "", false }
	mk := func(ps string, desc *font.Descriptor) font.Instance {
		return (&dict.Type1{PostScriptName: ps, Descriptor: desc, Encoding: enc}).MakeFont()
	}
	box := georect.Rect{LLx: -100, LLy: -300, URx: 1000, URy: 900}

	// every usable extent contains the baseline
	cases := []struct {
		name string
		f    font.Instance
	}{
		{"descriptor", mk("Test", &font.Descriptor{FontName: "Test", Descent: -250, Ascent: 750})},
		{"positive descent", mk("Test", &font.Descriptor{FontName: "Test", Descent: 212, Ascent: 905})},
		{"bbox above baseline", mk("Test", &font.Descriptor{FontName: "Test", FontBBox: georect.Rect{LLy: 100, URy: 900}})},
		{"bbox", mk("Test", &font.Descriptor{FontName: "Test", FontBBox: box})},
		{"standard font", mk("Helvetica", nil)},
		{"fallback font", extract.StandardFontFallback("Helv")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, a, ok := fontExtent(c.f)
			if !ok {
				t.Fatal("no usable extent")
			}
			if !(d <= 0 && 0 < a) {
				t.Errorf("extent [%g, %g] does not contain the baseline", d, a)
			}
		})
	}

	t.Run("positive descent keeps ascent", func(t *testing.T) {
		d, a, _ := fontExtent(mk("Test", &font.Descriptor{FontName: "Test", Descent: 212, Ascent: 905}))
		if a < 0.905 || d > 0 {
			t.Errorf("got [%g, %g]", d, a)
		}
	})

	t.Run("huge extent", func(t *testing.T) {
		d, a, ok := fontExtent(mk("Test", &font.Descriptor{FontName: "Test", Descent: -2.5e5, Ascent: 9e6}))
		if !ok || d > -250 || a < 9000 {
			t.Errorf("got [%g, %g], %v", d, a, ok)
		}
	})

	t.Run("skips unusable sources", func(t *testing.T) {
		inf := &font.Descriptor{FontName: "Test", Descent: math.Inf(-1), Ascent: 700, FontBBox: box}
		d, a, ok := fontExtent(mk("Test", inf))
		if !ok || d > -0.3 || a < 0.9 {
			t.Errorf("got [%g, %g], %v", d, a, ok)
		}
	})

	t.Run("unusable no descriptor", func(t *testing.T) {
		if _, _, ok := fontExtent(mk("Test", nil)); ok {
			t.Error("unexpected usable extent")
		}
	})

	for name, f := range map[string]font.Instance{
		"empty":           mk("Test", &font.Descriptor{FontName: "Test"}),
		"nan bbox":        mk("Test", &font.Descriptor{FontName: "Test", FontBBox: georect.Rect{LLy: math.NaN(), URy: 1}}),
		"zero ascent":     mk("Test", &font.Descriptor{FontName: "Test", Descent: -200}),
		"negative ascent": mk("Test", &font.Descriptor{FontName: "Test", Descent: -500, Ascent: -100}),
		"bbox below baseline": mk("Test", &font.Descriptor{FontName: "Test",
			FontBBox: georect.Rect{LLx: 0, LLy: -500, URx: 1000, URy: -100}}),
	} {
		t.Run("unusable "+name, func(t *testing.T) {
			if _, _, ok := fontExtent(f); ok {
				t.Error("unexpected usable extent")
			}
		})
	}
}

// TestGlyphBoxesBaseline checks that a positive descent in the font
// descriptor does not lift the glyph box off the baseline.
func TestGlyphBoxesBaseline(t *testing.T) {
	f := (&dict.Type1{
		PostScriptName: "Test",
		Descriptor:     &font.Descriptor{FontName: "Test", Descent: 212, Ascent: 905},
		Encoding:       func(byte) (string, bool) { return "A", true },
	}).MakeFont()
	got, ok := boxOf(t, newGlyphBoxes(f), font.Code{CID: 2, Width: 0.5})
	if !ok || got.LLy > -0.212 || got.URy < 0.905 {
		t.Errorf("got %v, %v", got, ok)
	}
}

// TestGlyphBoxesSubstituted checks that the glyphs of a font which a viewer
// substitutes cover the range of substitute glyphs, whatever the font
// descriptor says.
func TestGlyphBoxesSubstituted(t *testing.T) {
	enc := func(byte) (string, bool) { return "A", true }
	descs := map[string]*font.Descriptor{
		"empty":       {FontName: "Test"},
		"zero ascent": {FontName: "Test", Descent: -200},
		"tiny":        {FontName: "Test", Descent: -1, Ascent: 1, FontBBox: georect.Rect{LLx: 0, LLy: -1, URx: 1, URy: 1}},
	}
	cases := []struct {
		width float64
		want  georect.Rect
	}{
		{0, georect.Rect{LLx: -0.5, LLy: -0.5, URx: 1.5, URy: 1.5}},
		{0.6, georect.Rect{LLx: -0.5, LLy: -0.5, URx: 1.5, URy: 1.5}},
		{3, georect.Rect{LLx: -1.5, LLy: -0.5, URx: 4.5, URy: 1.5}},
		{-2, georect.Rect{LLx: -3, LLy: -0.5, URx: 3, URy: 1.5}},
	}
	for name, desc := range descs {
		f := (&dict.Type1{PostScriptName: "Test", Descriptor: desc, Encoding: enc}).MakeFont()
		g := newGlyphBoxes(f)
		for _, c := range cases {
			got, ok := boxOf(t, g, font.Code{CID: 2, Width: c.width})
			if !ok || !got.Covers(c.want) {
				t.Errorf("%s, width %g: box %v does not cover %v", name, c.width, got, c.want)
			}
		}
	}
}

// TestGlyphBoxesUnion checks that the font bounding box adds to the ascent
// and descent, and does not replace them.
func TestGlyphBoxesUnion(t *testing.T) {
	enc := func(byte) (string, bool) { return "A", true }
	mk := func(desc, asc float64, bbox georect.Rect) font.Instance {
		return (&dict.Type1{
			PostScriptName: "Test",
			Descriptor:     &font.Descriptor{FontName: "Test", Descent: desc, Ascent: asc, FontBBox: bbox},
			Encoding:       enc,
		}).MakeFont()
	}
	small := georect.Rect{LLx: 0, LLy: -100, URx: 500, URy: 500}
	got, _ := boxOf(t, newGlyphBoxes(mk(-700, 1800, small)), font.Code{CID: 2, Width: 0.5})
	if got.LLy > -0.7 || got.URy < 1.8 {
		t.Errorf("box %v is smaller than the ascent and descent", got)
	}
	large := georect.Rect{LLx: 0, LLy: -900, URx: 500, URy: 2000}
	got, _ = boxOf(t, newGlyphBoxes(mk(-700, 1800, large)), font.Code{CID: 2, Width: 0.5})
	if got.LLy > -0.9 || got.URy < 2.0 {
		t.Errorf("box %v is smaller than the font bounding box", got)
	}
}

// TestGlyphBoxesBadProgram checks that a font whose embedded program cannot
// be read has no bound, however plausible its descriptor is.
func TestGlyphBoxesBadProgram(t *testing.T) {
	garbage := func(typ glyphdata.Type) *glyphdata.Stream {
		return &glyphdata.Stream{Type: typ, WriteTo: func(w io.Writer, _ *glyphdata.Lengths) error {
			_, err := w.Write([]byte("not a font program"))
			return err
		}}
	}
	for _, typ := range []glyphdata.Type{glyphdata.Type1, glyphdata.CFFSimple, glyphdata.TrueType} {
		f := (&dict.Type1{
			PostScriptName: "Test",
			Descriptor: &font.Descriptor{
				FontName: "Test", Descent: -200, Ascent: 700,
				FontBBox: georect.Rect{LLx: 0, LLy: -200, URx: 700, URy: 700},
			},
			Encoding: func(byte) (string, bool) { return "A", true },
			FontFile: garbage(typ),
		}).MakeFont()
		g := newGlyphBoxes(f)
		if _, bounded, _ := g.points(&font.Code{CID: 2, Width: 0.5}, noCharge); bounded {
			t.Errorf("glyph of a font with a broken %v program has a bound", typ)
		}
	}
}

// TestType3InkOutsideBox checks that a region over the ink of a d1 glyph
// removes the glyph, also where the ink lies outside the box of the d1
// operator.
func TestType3InkOutsideBox(t *testing.T) {
	b := builder.New(content.Glyph, nil, pdf.V2_0)
	b.Type3UncoloredGlyph(500, 0, 0, 0, 500, 700)
	b.Rectangle(0, 0, 500, 1500)
	b.Fill()
	stream, err := b.Harvest()
	if err != nil {
		t.Fatal(err)
	}
	F := &type3.Font{
		Glyphs:     []*type3.Glyph{{}, {Name: "A", Content: stream}},
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
	}
	inst, err := F.New()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		region pdf.Array
		want   string
	}{
		{"on the ink", rect(50, 113, 56, 117), ""},
		{"above the ink", rect(50, 119, 56, 125), "A"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := fmt.Sprintf("BT /T3 12 Tf 50 100 Td %s Tj ET", hexShow(t, inst, "A"))
			o := &srcOpts{fonts: map[pdf.Name]font.Instance{"T3": inst}}
			in := textSourceOpt(t, data, o, pdf.Dict{"Rect": c.region})
			got := glyphString(pageGlyphs(t, writeAndRead(t, in, nil, "")))
			if got != c.want {
				t.Errorf("got text %q, want %q", got, c.want)
			}
		})
	}
}

// boxOf returns the bounding rectangle of the box of a glyph.
func boxOf(t testing.TB, g *glyphBoxes, c font.Code) (georect.Rect, bool) {
	t.Helper()
	pts, ok, err := g.points(&c, noCharge)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		return georect.Rect{}, false
	}
	r := georect.Rect{LLx: pts[0].X, LLy: pts[0].Y, URx: pts[0].X, URy: pts[0].Y}
	for _, p := range pts {
		r.Add(p.X, p.Y)
	}
	return r, true
}

func TestGlyphBoxesInk(t *testing.T) {
	enc := func(code byte) (string, bool) {
		switch code {
		case 1:
			return "Aring", true
		case 2:
			return "period", true
		}
		return "", false
	}
	f := (&dict.Type1{
		PostScriptName: "Helvetica",
		Descriptor:     &font.Descriptor{FontName: "Helvetica"},
		Encoding:       enc,
	}).MakeFont()
	g := newGlyphBoxes(f)
	if g.ink == nil {
		t.Fatal("no glyph bounds for a standard font")
	}

	// the ink of Aring rises above the ascent
	aring, _ := boxOf(t, g, font.Code{CID: 2, Width: 0.667})
	if aring.URy < 0.929 {
		t.Errorf("box of Aring ends at %g, want at least 0.929", aring.URy)
	}
	// the advance box covers the ascent and descent for blank-ish glyphs
	period, _ := boxOf(t, g, font.Code{CID: 3, Width: 0.278})
	if period.LLy > -0.207 || period.URy < 0.718 || period.URx < 0.278 {
		t.Errorf("box of period is %v, want to cover the advance box", period)
	}
}

func TestGlyphBoxesFallback(t *testing.T) {
	bbox := georect.Rect{LLx: -700, LLy: -800, URx: 1900, URy: 1700}
	enc := func(byte) (string, bool) { return "A", true }
	f := (&dict.Type1{
		PostScriptName: "Test",
		Descriptor:     &font.Descriptor{FontName: "Test", Descent: -200, Ascent: 700, FontBBox: bbox},
		Encoding:       enc,
	}).MakeFont()
	g := newGlyphBoxes(f)
	if g.ink != nil {
		t.Fatal("unexpected glyph bounds")
	}

	// a font bounding box beyond the range of substitute glyphs widens the
	// box
	got, _ := boxOf(t, g, font.Code{CID: 2, Width: 0.5})
	want := georect.Rect{LLx: -0.7, LLy: -0.8, URx: 1.9, URy: 1.7}
	if diff := cmp.Diff(want, got, cmpopts.EquateApprox(0, 1e-12)); diff != "" {
		t.Errorf("wrong box (-want +got):\n%s", diff)
	}

	// so do ascent and descent
	f = (&dict.Type1{
		PostScriptName: "Test",
		Descriptor:     &font.Descriptor{FontName: "Test", Descent: -600, Ascent: 1600},
		Encoding:       enc,
	}).MakeFont()
	got, _ = boxOf(t, newGlyphBoxes(f), font.Code{CID: 2, Width: 0.5})
	want = georect.Rect{LLx: -0.5, LLy: -0.6, URx: 1.5, URy: 1.6}
	if diff := cmp.Diff(want, got, cmpopts.EquateApprox(0, 1e-12)); diff != "" {
		t.Errorf("wrong box (-want +got):\n%s", diff)
	}
}

// TestEmbeddedInk checks that the ink bounds of embedded fonts agree with
// the glyph bounding boxes of the font programs.
func TestEmbeddedInk(t *testing.T) {
	const text = "Hgy,"
	tt, ot, otCID := makefont.TrueType(), makefont.OpenType(), makefont.OpenTypeCID()

	makers := map[string]struct {
		src *sfnt.Font
		mk  func() (font.Layouter, error)
	}{
		"truetype": {tt, func() (font.Layouter, error) { return truetype.NewSimple(tt, nil) }},
		"truetype composite": {tt, func() (font.Layouter, error) {
			return truetype.NewComposite(tt, &truetype.OptionsComposite{CMap: cmap.IdentityH})
		}},
		"truetype vertical": {tt, func() (font.Layouter, error) {
			return truetype.NewComposite(tt, &truetype.OptionsComposite{CMap: cmap.IdentityV})
		}},
		"cff": {ot, func() (font.Layouter, error) { return cff.NewSimple(ot, nil) }},
		"cff composite": {ot, func() (font.Layouter, error) {
			return cff.NewComposite(ot, &cff.OptionsComposite{CMap: cmap.IdentityH})
		}},
		"cff CID": {otCID, func() (font.Layouter, error) {
			return cff.NewComposite(otCID, &cff.OptionsComposite{CMap: cmap.IdentityH})
		}},
		"opentype": {ot, func() (font.Layouter, error) { return opentype.NewSimple(ot, nil) }},
	}
	for name, m := range makers {
		t.Run(name, func(t *testing.T) {
			F, err := m.mk()
			if err != nil {
				t.Fatal(err)
			}
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			ref, err := rm.Embed(F)
			if err != nil {
				t.Fatal(err)
			}
			var codes []byte
			var gids []sglyph.ID
			for _, g := range F.Layout(nil, 1, text).Seq {
				code, _ := F.Encode(g.GID, g.Text)
				codes = F.Codec().AppendCode(codes, code)
				gids = append(gids, g.GID)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}
			inst, err := extract.Font(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
			if err != nil {
				t.Fatal(err)
			}

			ink := newInkFunc(inst.FontInfo())
			if ink == nil {
				t.Fatal("no glyph bounds")
			}
			upem := float64(m.src.UnitsPerEm)
			var charged int64
			i := 0
			for c := range inst.Codes(codes) {
				b := m.src.GlyphBBox(gids[i])
				want := georect.Rect{LLx: b.LLx / upem, LLy: b.LLy / upem, URx: b.URx / upem, URy: b.URy / upem}
				got, err := ink(c.CID, func(n int64) error {
					charged += n
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				if math.Abs(got.LLx-want.LLx) > 2e-3 || math.Abs(got.LLy-want.LLy) > 2e-3 ||
					math.Abs(got.URx-want.URx) > 2e-3 || math.Abs(got.URy-want.URy) > 2e-3 {
					t.Errorf("glyph %d: got %v, want %v", i, got, want)
				}
				i++
			}
			if i != len(gids) {
				t.Errorf("read %d codes, want %d", i, len(gids))
			}
			if charged <= 0 {
				t.Error("glyph analysis was not charged to the budget")
			}
		})
	}
}

func TestEmbeddedInkType1(t *testing.T) {
	F, err := type1.New(makefont.Type1(), makefont.AFM())
	if err != nil {
		t.Fatal(err)
	}
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	var codes []byte
	for _, g := range F.Layout(nil, 1, "H").Seq {
		code, _ := F.Encode(g.GID, g.Text)
		codes = F.Codec().AppendCode(codes, code)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	inst, err := extract.Font(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	ink := newInkFunc(inst.FontInfo())
	if ink == nil {
		t.Fatal("no glyph bounds")
	}
	var charged int64
	for c := range inst.Codes(codes) {
		got, err := ink(c.CID, func(n int64) error {
			charged += n
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if charged <= 0 {
			t.Error("glyph analysis was not charged to the budget")
		}
		if !(got.URy > 0.3 && got.URy < 1.2 && got.URx > got.LLx) {
			t.Errorf("implausible bounds %v for H", got)
		}
	}
}

// TestGlyphBoxesNoExtent checks that the glyphs of a font without usable
// ascent and descent are bounded by their ink, where the font program gives
// the ink of individual glyphs.
func TestGlyphBoxesNoExtent(t *testing.T) {
	F, err := truetype.NewSimple(makefont.TrueType(), nil)
	if err != nil {
		t.Fatal(err)
	}
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	var codes []byte
	for _, g := range F.Layout(nil, 1, "H").Seq {
		code, _ := F.Encode(g.GID, g.Text)
		codes = F.Codec().AppendCode(codes, code)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	inst, err := extract.Font(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	desc, _ := fontDescriptor(inst)
	if desc == nil {
		t.Fatal("no font descriptor")
	}
	desc.Ascent, desc.Descent, desc.FontBBox = 0, 0, georect.Rect{}
	if _, _, ok := fontExtent(inst); ok {
		t.Fatal("unexpected usable extent")
	}

	g := newGlyphBoxes(inst)
	for c := range inst.Codes(codes) {
		ink, err := g.ink(c.CID, noCharge)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := boxOf(t, g, c)
		if !ok {
			t.Fatal("glyph has no bound")
		}
		if !got.Covers(ink) {
			t.Errorf("box %v does not cover the ink %v", got, ink)
		}
		if got.LLx > 0 || got.URx < c.Width || got.LLy > 0 || got.URy < 0 {
			t.Errorf("box %v does not cover the advance on the baseline", got)
		}
		if got.URy > 2 || got.LLy < -1 {
			t.Errorf("box %v is not bounded by the ink", got)
		}
	}
}

// TestSubstitutedText checks that glyphs of fonts which are not embedded are
// removed where a substitute glyph can paint under a region, whatever the
// font dictionary says about the glyph extents.
func TestSubstitutedText(t *testing.T) {
	descriptor := func(name string, ascent, descent int) pdf.Dict {
		return pdf.Dict{
			"Type": pdf.Name("FontDescriptor"), "FontName": pdf.Name(name),
			"Flags": pdf.Integer(32), "FontBBox": rect(0, 0, 0, 0),
			"ItalicAngle": pdf.Integer(0), "Ascent": pdf.Integer(ascent),
			"Descent": pdf.Integer(descent), "CapHeight": pdf.Integer(700),
			"StemV": pdf.Integer(80),
		}
	}
	simple := func(subtype string, widths pdf.Array, desc pdf.Dict) pdf.Dict {
		d := pdf.Dict{
			"Type": pdf.Name("Font"), "Subtype": pdf.Name(subtype),
			"BaseFont": desc["FontName"], "FontDescriptor": desc,
		}
		if widths != nil {
			d["FirstChar"] = pdf.Integer(65)
			d["LastChar"] = pdf.Integer(64 + len(widths))
			d["Widths"] = widths
		}
		return d
	}
	vertical := pdf.Dict{
		"Type": pdf.Name("Font"), "Subtype": pdf.Name("Type0"),
		"BaseFont": pdf.Name("FooMincho"), "Encoding": pdf.Name("Identity-V"),
		"DescendantFonts": pdf.Array{pdf.Dict{
			"Type": pdf.Name("Font"), "Subtype": pdf.Name("CIDFontType0"),
			"BaseFont": pdf.Name("FooMincho"),
			"CIDSystemInfo": pdf.Dict{
				"Registry": pdf.String("Adobe"), "Ordering": pdf.String("Japan1"),
				"Supplement": pdf.Integer(4),
			},
			"DW":             pdf.Integer(1000),
			"FontDescriptor": descriptor("FooMincho", 0, -120),
		}},
	}
	unknownGlyph := pdf.Dict{
		"Type": pdf.Name("Font"), "Subtype": pdf.Name("Type1"),
		"BaseFont":  pdf.Name("Helvetica"),
		"Encoding":  pdf.Dict{"Differences": pdf.Array{pdf.Integer(65), pdf.Name("afii10017")}},
		"FirstChar": pdf.Integer(65), "LastChar": pdf.Integer(65),
		"Widths": pdf.Array{pdf.Integer(0)},
	}

	cases := []struct {
		name    string
		font    pdf.Dict
		content string
		region  pdf.Array
		// kept lists the CIDs of the glyphs which are kept
		kept []cid.CID
	}{
		{
			// the region lies above the ascent of the font
			name:    "zero ascent",
			font:    simple("TrueType", pdf.Array{pdf.Integer(600), pdf.Integer(600), pdf.Integer(600)}, descriptor("FooSans", 0, -200)),
			content: "BT /F2 100 Tf 10 100 Td (ABC) Tj ET",
			region:  rect(10, 120, 200, 170),
		},
		{
			// the region lies to the right of the advance of A
			name:    "zero widths",
			font:    simple("TrueType", pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(0)}, descriptor("FooSans", 800, -200)),
			content: "BT /F2 100 Tf 10 100 Td [(A) -600 (B) -600 (C)] TJ ET",
			region:  rect(20, 100, 60, 150),
			kept:    []cid.CID{'C' + 1},
		},
		{
			name:    "no widths",
			font:    simple("Type1", nil, descriptor("FooSerif", 800, -200)),
			content: "BT /F2 100 Tf 10 100 Td 60 Tc (ABC) Tj ET",
			region:  rect(20, 100, 60, 150),
			kept:    []cid.CID{'C' + 1},
		},
		{
			// the second glyph is painted below its vertical origin
			name:    "vertical",
			font:    vertical,
			content: "BT /F2 100 Tf 100 250 Td <00010002> Tj ET",
			region:  rect(60, 60, 140, 100),
			kept:    []cid.CID{1},
		},
		{
			// the viewer can show the glyph from a font which has it
			name:    "glyph missing from standard font",
			font:    unknownGlyph,
			content: "BT /F2 100 Tf 10 100 Td (A) Tj ET",
			region:  rect(70, 110, 140, 150),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := &srcOpts{rawFonts: map[pdf.Name]pdf.Dict{"F2": c.font}}
			before := pageGlyphs(t, textSourceOpt(t, c.content, o))
			if len(before) == 0 {
				t.Fatal("test page shows no glyphs")
			}
			in := textSourceOpt(t, c.content, o, pdf.Dict{"Rect": c.region})
			var kept []cid.CID
			for _, g := range pageGlyphs(t, writeAndRead(t, in, nil, "")) {
				kept = append(kept, g.code.CID)
			}
			if diff := cmp.Diff(c.kept, kept); diff != "" {
				t.Errorf("wrong glyphs kept (-want +got):\n%s", diff)
			}
		})
	}
}

// TestGlyphBoxesNotdef checks that the box of a code whose CID the font
// lacks covers the glyph of the notdef CID, which a viewer shows instead.
func TestGlyphBoxesNotdef(t *testing.T) {
	F, err := truetype.NewComposite(makefont.TrueType(), &truetype.OptionsComposite{CMap: cmap.IdentityH})
	if err != nil {
		t.Fatal(err)
	}
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	codes := codesOf(t, F, "H")
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	inst, err := extract.Font(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	var h font.Code
	for c := range inst.Codes(codes) {
		h = c
	}

	g := newGlyphBoxes(inst)
	ink, err := g.ink(h.CID, noCharge)
	if err != nil {
		t.Fatal(err)
	}
	if ink.IsZero() || !ink.IsFinite() {
		t.Fatalf("implausible ink %v for H", ink)
	}
	got, ok := boxOf(t, g, font.Code{CID: 0xFFF0, Notdef: h.CID})
	if !ok || !got.Covers(ink) {
		t.Errorf("box %v does not cover the ink %v of the notdef glyph", got, ink)
	}
}
