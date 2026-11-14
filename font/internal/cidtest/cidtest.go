// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package cidtest checks the glyph metrics in the CIDFont dictionaries
// written by the composite font writers.  It is used by their tests.
package cidtest

import (
	"math"
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// Font is a composite font as the writers produce it.
type Font interface {
	font.Layouter
	GlyphID(c cid.CID) (glyph.ID, bool)
}

// embedded is a font which was embedded and read back.
type embedded struct {
	// cidFont is the CIDFont dictionary as written.
	cidFont pdf.Dict

	width map[cid.CID]float64
	dw    float64
	dw2   dict.DefaultVMetrics
	w2    map[cid.CID]dict.VMetrics

	// fontFile is the embedded font program.
	fontFile *glyphdata.Stream

	// read is the font read back from the file.
	read font.Instance

	// s is the text, followed by the extra bytes.
	s pdf.String
}

// extraCodes are appended to the text shown.  With a two-byte code space
// they give three codes whose CIDs were never encoded and a final incomplete
// code; with the UTF-8 code space they give codes which were never
// allocated, an invalid code and an incomplete three-byte code.
var extraCodes = []byte{0x01, 0x23, 'Z', 0xFF, 0x05, 0xE3, 0x81}

// embed embeds F, shows text followed by the given extra bytes, and reads the
// font back.
func embed(t *testing.T, F Font, text string, extra []byte) *embedded {
	t.Helper()

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
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
	s = append(s, extra...)
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	res := &embedded{s: s}

	c := pdf.NewCursor(w)
	fontDict, err := c.Dict(ref)
	if err != nil {
		t.Fatal(err)
	}
	a, err := c.Array(fontDict["DescendantFonts"])
	if err != nil || len(a) != 1 {
		t.Fatalf("invalid DescendantFonts %v (%v)", fontDict["DescendantFonts"], err)
	}
	res.cidFont, err = c.Dict(a[0])
	if err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	d, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	switch d := d.(type) {
	case *dict.CIDFontType0:
		res.width, res.dw, res.dw2, res.w2 = d.Width, d.DefaultWidth, d.DefaultVMetrics, d.VMetrics
		res.fontFile = d.FontFile
	case *dict.CIDFontType2:
		res.width, res.dw, res.dw2, res.w2 = d.Width, d.DefaultWidth, d.DefaultVMetrics, d.VMetrics
		res.fontFile = d.FontFile
	default:
		t.Fatalf("wrong font dictionary type: %T", d)
	}
	res.read = d.MakeFont()

	return res
}

// defaultExtra returns code 0, followed by the bytes of extraCodes.
func defaultExtra(F Font) []byte {
	return append(F.Codec().AppendCode(nil, 0), extraCodes...)
}

// ZeroAdvance gives the glyph info maps r to a vertical advance of zero, so
// that a text using r needs a W2 entry with a zero vertical displacement.
func ZeroAdvance(t *testing.T, info *sfnt.Font, r rune) {
	t.Helper()
	lookup, err := info.CMapTable.GetBest()
	if err != nil {
		t.Fatal(err)
	}
	gid := lookup.Lookup(r)
	if gid == 0 || info.Vertical == nil {
		t.Fatalf("no vertical metrics for %q", r)
	}
	info.Vertical.Heights[gid] = 0
}

// CheckVertical embeds F, which must use vertical writing, shows text, code
// 0 and some codes whose CIDs were never encoded, and reads the font back.
// It checks that DW2 holds the vertical metrics of glyph 0, that DW2 and W2
// give every CID shown the vertical metrics info gives its glyph, and that the
// font read back reports the same [font.Code] for every code as F does.
//
// The text must use a glyph whose vertical metrics differ from those of
// glyph 0.
func CheckVertical(t *testing.T, F Font, info *sfnt.Font, text string) {
	t.Helper()

	if F.WritingMode() != font.Vertical {
		t.Fatal("font is not for vertical writing")
	}
	e := embed(t, F, text, defaultExtra(F))

	if want := want(info, 0); e.dw2 != want {
		t.Errorf("DW2 is %v, want %v", e.dw2, want)
	}
	if len(e.w2) == 0 {
		t.Error("no W2 entries")
	}

	for c := range e.read.Codes(e.s) {
		gid, _ := F.GlyphID(c.CID)
		v := want(info, gid)

		w, ok := e.width[c.CID]
		if !ok {
			w = e.dw
		}
		got, ok := e.w2[c.CID]
		if !ok {
			got = dict.VMetrics{OffsX: w / 2, OffsY: e.dw2.OffsY, DeltaY: e.dw2.DeltaY}
		}
		wantVM := dict.VMetrics{OffsX: w / 2, OffsY: v.OffsY, DeltaY: v.DeltaY}
		if got != wantVM {
			t.Errorf("CID %d: vertical metrics are %v, want %v", c.CID, got, wantVM)
		}

		gotV := vertical(c)
		wantV := [3]float64{v.DeltaY / 1000, w / 2 / 1000, v.OffsY / 1000}
		if gotV != wantV {
			t.Errorf("CID %d: vertical metrics read back are %v, want %v",
				c.CID, gotV, wantV)
		}
	}

	CheckCodes(t, F, e.read, e.s)
	checkTrailingSpace(t, F, e.read)
}

// CheckNotdefGlyph embeds F, which must have been made from info, and shows
// text.  It checks that CID 0 has the width of glyph 0 of info, that this
// is the default width, and that glyph 0 of the embedded font program has
// the bounding box of glyph 0 of info.
//
// The text must not be empty.
func CheckNotdefGlyph(t *testing.T, F Font, info *sfnt.Font, text string) {
	t.Helper()

	e := embed(t, F, text, defaultExtra(F))

	wantWidth := math.Round(info.GlyphWidthPDF(0))
	if e.dw != wantWidth {
		t.Errorf("DW is %g, want %g", e.dw, wantWidth)
	}
	if w, ok := e.width[0]; ok && w != wantWidth {
		t.Errorf("CID 0 has width %g, want %g", w, wantWidth)
	}

	if e.fontFile == nil {
		t.Fatal("no font file")
	}
	var got, want rect.Rect
	switch e.fontFile.Type {
	case glyphdata.CFF, glyphdata.CFFSimple:
		sub, err := cffglyphs.FromStream(e.fontFile)
		if err != nil {
			t.Fatal(err)
		}
		got = sub.Outlines.GlyphBBox(matrix.Identity, 0)
		want = info.AsCFF().Outlines.GlyphBBox(matrix.Identity, 0)
	default:
		sub, err := sfntglyphs.FromStream(e.fontFile)
		if err != nil {
			t.Fatal(err)
		}
		got = sub.GlyphBBox(0)
		want = info.GlyphBBox(0)
	}
	if got != want {
		t.Errorf("glyph 0 has bounding box %v, want %v", got, want)
	}
}

// CheckHorizontal embeds F, which must use horizontal writing, shows text,
// code 0 and some codes whose CIDs were never encoded, and reads the font
// back.  It checks that the CIDFont dictionary has neither DW2 nor W2, that
// no code has a vertical advance, and that the font read back reports the
// same [font.Code] for every code as F does.
func CheckHorizontal(t *testing.T, F Font, text string) {
	t.Helper()

	if F.WritingMode() != font.Horizontal {
		t.Fatal("font is not for horizontal writing")
	}
	e := embed(t, F, text, defaultExtra(F))

	for _, key := range []pdf.Name{"DW2", "W2"} {
		if obj, ok := e.cidFont[key]; ok {
			t.Errorf("CIDFont dictionary has /%s %v", key, obj)
		}
	}
	if e.dw2 != dict.DefaultVMetricsDefault || e.w2 != nil {
		t.Errorf("vertical metrics read back are %v, %v", e.dw2, e.w2)
	}
	for c := range F.Codes(e.s) {
		if v := vertical(c); v != [3]float64{} {
			t.Errorf("CID %d: vertical metrics are %v, want none", c.CID, v)
		}
	}

	CheckCodes(t, F, e.read, e.s)
	checkTrailingSpace(t, F, e.read)
}

// CheckCodes checks that the font written, F, and the font read back, read,
// report the same [font.Code] for every code in s.
func CheckCodes(t *testing.T, F, read font.Instance, s pdf.String) {
	t.Helper()

	var written []font.Code
	for c := range F.Codes(s) {
		written = append(written, c)
	}
	i := 0
	for c := range read.Codes(s) {
		if i >= len(written) {
			t.Fatal("font read back gives more codes than the writer")
		}
		if c != written[i] {
			t.Errorf("code %d: %+v when written, %+v when read", i, written[i], c)
		}
		i++
	}
	if i != len(written) {
		t.Errorf("font read back gives %d codes, the writer %d", i, len(written))
	}
}

// checkTrailingSpace checks code 0 followed by a lone 0x20 byte.  With a
// multi-byte code space the lone byte is an incomplete code, which must not
// count as a space.
func checkTrailingSpace(t *testing.T, F Font, read font.Instance) {
	t.Helper()
	CheckCodes(t, F, read, append(F.Codec().AppendCode(nil, 0), 0x20))
}

// vertical returns the vertical advance and the position vector of c.
func vertical(c font.Code) [3]float64 {
	return [3]float64{c.VerticalAdvance, c.VerticalOriginX, c.VerticalOriginY}
}

// want returns the vertical metrics of a glyph as a font writer stores them:
// the values of info, rounded to integers.
func want(info *sfnt.Font, gid glyph.ID) dict.DefaultVMetrics {
	advance, originY := info.GlyphVerticalPDF(gid)
	return dict.DefaultVMetrics{OffsY: math.Round(originY), DeltaY: -math.Round(advance)}
}

// NotdefText is the text which [CheckNotdef] shows.
const NotdefText = "HELLO"

// notdefCase is a code shown by [CheckNotdef], with the CID and the notdef
// CID a reader selects for it.
type notdefCase struct {
	name        string
	code        []byte
	cid, notdef cid.CID
}

// notdefCases are the codes [CheckNotdef] shows after [NotdefText], for the
// CMap of [NotdefCMap].  The last code is incomplete.
var notdefCases = []notdefCase{
	{"mapped, encoded", []byte{'H'}, 107, 0},
	{"mapped, not encoded", []byte{'Z'}, 125, 0},
	{"mapped, no glyph", []byte{0x7E}, 9999, 0},
	{"notdef, CID encoded", []byte{0x05}, 104, 104},
	{"notdef, CID not encoded", []byte{0x15}, 120, 120},
	{"valid, unmapped", []byte{0x61}, 0, 0},
	{"valid, unmapped space", []byte{0x20}, 0, 0},
	{"mapped two bytes", []byte{0x80, 0x00}, 111, 0},
	{"valid, unmapped two bytes", []byte{0x80, 0x01}, 0, 0},
	{"invalid", []byte{0x80, 0x50}, 0, 0},
	// same code value as 8000
	{"invalid, incomplete", []byte{0x80}, 0, 0},
}

// NotdefCMap returns a CMap with notdef mappings for the given writing
// mode, and a mapping from the glyphs of info to the CIDs of its character
// collection, for use with [CheckNotdef].
//
// The CMap has one-byte codes 00..7F and two-byte codes 8000..803F.  The
// codes 'A'..'Z' map to the CIDs 100..125, 7E to CID 9999 which has no
// glyph, and 8000 to CID 111 ('L').  The notdef mappings send 00..0F to
// CID 104 ('E', which [NotdefText] uses) and 10..1F to CID 120 ('U', which
// it does not use).
func NotdefCMap(t *testing.T, info *sfnt.Font, wMode font.WritingMode) (*cmap.File, cmap.GIDToCID) {
	t.Helper()
	return notdefCMap(t, info, wMode, nil)
}

// SharedCIDCMap returns the CMap and the glyph mapping of [NotdefCMap],
// except that the glyph for 'i' shares CID 125 with the glyph for 'Z', for
// use with [CheckSharedCID].
func SharedCIDCMap(t *testing.T, info *sfnt.Font, wMode font.WritingMode) (*cmap.File, cmap.GIDToCID) {
	t.Helper()
	return notdefCMap(t, info, wMode, map[rune]cid.CID{'i': 125})
}

func notdefCMap(t *testing.T, info *sfnt.Font, wMode font.WritingMode, extra map[rune]cid.CID) (*cmap.File, cmap.GIDToCID) {
	t.Helper()
	lookup, err := info.CMapTable.GetBest()
	if err != nil {
		t.Fatal(err)
	}
	ros := &cid.SystemInfo{Registry: "Test", Ordering: "Private"}
	table := make(map[glyph.ID]cid.CID)
	for r := 'A'; r <= 'Z'; r++ {
		table[lookup.Lookup(r)] = cid.CID(100 + r - 'A')
	}
	for r, c := range extra {
		table[lookup.Lookup(r)] = c
	}

	f := &cmap.File{
		Name:  "Test-Private-Notdef",
		ROS:   ros,
		WMode: wMode,
		CodeSpaceRange: charcode.CodeSpaceRange{
			{Low: []byte{0x00}, High: []byte{0x7F}},
			{Low: []byte{0x80, 0x00}, High: []byte{0x80, 0x3F}},
		},
		CIDRanges: []cmap.Range{{First: []byte{'A'}, Last: []byte{'Z'}, Value: 100}},
		CIDSingles: []cmap.Single{
			{Code: []byte{0x7E}, Value: 9999},
			{Code: []byte{0x80, 0x00}, Value: 111},
		},
		NotdefRanges: []cmap.Range{
			{First: []byte{0x00}, Last: []byte{0x0F}, Value: 104},
			{First: []byte{0x10}, Last: []byte{0x1F}, Value: 120},
		},
	}
	return f, cmap.NewGIDToCIDFromMap(ros, table)
}

// CheckNotdef embeds F, which must have been created with the CMap and the
// glyph mapping of [NotdefCMap], shows [NotdefText] followed by codes which
// are mapped, mapped through notdef mappings, unmapped, invalid and
// incomplete, and reads the font back.  It checks that the font read back
// reports the same [font.Code] for every code as F does, that the codes
// select the expected CIDs, and that every code has the metrics of the CID
// it selects, as given by the W, DW, W2 and DW2 entries.  Since the CMap
// gives CID 0 no code, it also checks that glyph 0, which shows a character
// missing from the font, cannot be encoded.
func CheckNotdef(t *testing.T, F Font) {
	t.Helper()

	var extra []byte
	for _, c := range notdefCases {
		extra = append(extra, c.code...)
	}
	e := embed(t, F, NotdefText, extra)
	CheckCodes(t, F, e.read, e.s)

	var got []font.Code
	for c := range e.read.Codes(e.s) {
		got = append(got, c)
	}
	if len(got) != len(NotdefText)+len(notdefCases) {
		t.Fatalf("got %d codes, want %d", len(got), len(NotdefText)+len(notdefCases))
	}
	for i, c := range notdefCases {
		g := got[len(NotdefText)+i]
		if g.CID != c.cid || g.Notdef != c.notdef {
			t.Errorf("%s: CID %d, notdef %d; want %d, %d", c.name, g.CID, g.Notdef, c.cid, c.notdef)
		}
	}

	vertical := F.WritingMode() == font.Vertical
	for i, g := range got {
		w, ok := e.width[g.CID]
		if !ok {
			w = e.dw
		}
		if g.Width != w/1000 {
			t.Errorf("code %d (CID %d): width %g, want %g", i, g.CID, g.Width, w/1000)
		}
		if !vertical {
			continue
		}
		v, ok := e.w2[g.CID]
		if !ok {
			v = dict.VMetrics{OffsX: w / 2, OffsY: e.dw2.OffsY, DeltaY: e.dw2.DeltaY}
		}
		if g.VerticalAdvance != v.DeltaY/1000 || g.VerticalOriginY != v.OffsY/1000 {
			t.Errorf("code %d (CID %d): vertical metrics %g %g, want %v",
				i, g.CID, g.VerticalAdvance, g.VerticalOriginY, v)
		}
	}

	if c, ok := F.Encode(0, "�"); ok {
		s := F.Codec().AppendCode(nil, c)
		for code := range e.read.Codes(s) {
			t.Errorf("glyph 0 encoded as % x, which selects CID %d", []byte(s), code.CID)
		}
	}
}

// CheckSharedCID checks a font F created with the CMap and the glyph mapping
// of [SharedCIDCMap], where two glyphs of info with different metrics share
// a CID.  The embedded font has one glyph for the CID, so whichever of the
// two glyphs is encoded, and in whichever order, the code must have the
// metrics of the glyph written for the CID.  It also checks that the font
// read back reports the same [font.Code] as F does.
func CheckSharedCID(t *testing.T, F Font, info *sfnt.Font) {
	t.Helper()

	const shared cid.CID = 125
	lookup, err := info.CMapTable.GetBest()
	if err != nil {
		t.Fatal(err)
	}
	written, ok := F.GlyphID(shared)
	if !ok {
		t.Fatalf("no glyph for CID %d", shared)
	}
	gids := []glyph.ID{lookup.Lookup('i'), lookup.Lookup('Z')}
	if gids[0] == written {
		gids[0], gids[1] = gids[1], gids[0]
	}
	if gids[1] != written || info.GlyphWidthPDF(gids[0]) == info.GlyphWidthPDF(gids[1]) {
		t.Fatal("test assumes two glyphs with different widths for the shared CID")
	}

	// the glyph not written for the CID is encoded first
	codec := F.Codec()
	var s pdf.String
	for i, gid := range gids {
		c, ok := F.Encode(gid, string(rune('a'+i)))
		if !ok {
			t.Fatalf("cannot encode glyph %d", gid)
		}
		s = codec.AppendCode(s, c)
	}

	wantWidth := math.Round(info.GlyphWidthPDF(written)) / 1000
	wantV := want(info, written)
	vertical := F.WritingMode() == font.Vertical
	for c := range F.Codes(s) {
		if c.CID != shared {
			t.Errorf("code selects CID %d, want %d", c.CID, shared)
		}
		if c.Width != wantWidth {
			t.Errorf("width %g, want %g", c.Width, wantWidth)
		}
		if vertical && (c.VerticalAdvance != wantV.DeltaY/1000 || c.VerticalOriginY != wantV.OffsY/1000) {
			t.Errorf("vertical metrics %g %g, want %v", c.VerticalAdvance, c.VerticalOriginY, wantV)
		}
	}

	e := embed(t, F, "", nil)
	CheckCodes(t, F, e.read, s)
}
