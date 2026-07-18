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

package textextract

import (
	"bytes"
	"io"
	"testing"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

func TestGlyphNameMappingUnsupportedFont(t *testing.T) {
	// fonts whose FontInfo type is unsupported should give nil
	f := &mockFontInstance{}
	got := GlyphNameMapping(f)
	if got != nil {
		t.Errorf("expected nil for unsupported font, got %v", got)
	}
}

func TestGlyphNameMappingNilFontFile(t *testing.T) {
	// FontInfoGlyfEmbedded with nil FontFile should give nil
	f := &mockFontInfoGlyfEmbedded{
		fontInfo: &dict.FontInfoGlyfEmbedded{
			FontFile: nil,
		},
	}
	got := GlyphNameMapping(f)
	if got != nil {
		t.Errorf("expected nil for nil FontFile, got %v", got)
	}
}

// TestGlyphNameMappingSimpleCFFBuiltin checks that codes mapped to
// [encoding.UseBuiltin] are resolved via the embedded CFF font's built-in
// encoding.  This is the fallback used for Type 1 / Type 1C fonts whose
// /Encoding entry has a Differences array but no BaseEncoding.
func TestGlyphNameMappingSimpleCFFBuiltin(t *testing.T) {
	stream := makeCFFStream(t, map[byte]string{
		'A': "A",
		'B': "B",
		'C': "fi", // mapped to fi ligature in built-in encoding
	})

	encDiff := map[byte]string{
		'X': "X", // a code with a /Differences override
	}
	enc := encoding.Simple(func(code byte) string {
		if name, ok := encDiff[code]; ok {
			return name
		}
		return encoding.UseBuiltin
	})

	f := &mockFontInfoSimple{
		fontInfo: &dict.FontInfoSimple{
			PostScriptName: "Test",
			FontFile:       stream,
			Encoding:       enc,
		},
	}
	got := GlyphNameMapping(f)

	cases := map[byte]string{
		'A': "A",
		'B': "B",
		'C': "ﬁ", // fi ligature
		'X': "X",
	}
	for code, want := range cases {
		if g := got[cid.CID(code)+1]; g != want {
			t.Errorf("code %d: got %q, want %q", code, g, want)
		}
	}
}

// TestGlyphNameMappingSimpleTrueTypeBuiltin checks that codes mapped to
// [encoding.UseBuiltin] are resolved through the embedded TrueType font's
// cmap and post tables (PDF spec 9.6.5.4).  This is the fallback used for
// symbolic simple TrueType fonts that ship without /Encoding or /ToUnicode.
func TestGlyphNameMappingSimpleTrueTypeBuiltin(t *testing.T) {
	fontInfo := makefont.TrueType() // Go-Regular, ships with (1,0) cmap + post names.
	stream := sfntglyphs.ToStream(fontInfo, glyphdata.TrueType)

	f := &mockFontInfoSimple{
		fontInfo: &dict.FontInfoSimple{
			PostScriptName: fontInfo.PostScriptName(),
			FontFile:       stream,
			Encoding:       encoding.Builtin,
			IsSymbolic:     true,
		},
	}
	got := GlyphNameMapping(f)

	// With IsSymbolic and encoding.Builtin, NewTrueTypeSelector falls
	// through methods D/B/E to method A (cmap 1,0 / Mac Roman) and looks
	// up the GID's name in the post table.
	cases := map[byte]string{
		'A':  "A",
		'a':  "a",
		0xAE: "Æ", // Mac Roman AE -> glyph "AE"
		0xE7: "Á", // Mac Roman Aacute -> glyph "Aacute"
		0xDE: "ﬁ", // Mac Roman fi -> glyph "uniFB01"
	}
	for code, want := range cases {
		if g := got[cid.CID(code)+1]; g != want {
			t.Errorf("code 0x%02X: got %q, want %q", code, g, want)
		}
	}
}

// TestGlyphNameMappingGlyfEmbeddedCFF2NoPanic checks that a FontFile3/OpenType
// stream labeled as glyf-flavored (TrueType/OpenTypeGlyf) but actually
// carrying CFF2 outlines (a malformed or mislabeled font, which sfnt.Read no
// longer rejects since go-sfnt C6) falls back to nil instead of panicking.
func TestGlyphNameMappingGlyfEmbeddedCFF2NoPanic(t *testing.T) {
	stream := makeCFF2Stream(t, glyphdata.TrueType)

	f := &mockFontInfoGlyfEmbedded{
		fontInfo: &dict.FontInfoGlyfEmbedded{
			PostScriptName: "Test",
			FontFile:       stream,
			CIDToGID:       []glyph.ID{0, 0},
		},
	}
	got := GlyphNameMapping(f)
	if got != nil {
		t.Errorf("expected nil for a CFF2-backed glyf stream, got %v", got)
	}
}

// TestGlyphNameMappingSimpleCFF2NoPanic mirrors the above for the simple-font
// path, which resolves [encoding.UseBuiltin] codes through
// [sfntglyphs.NewTrueTypeSelector].
func TestGlyphNameMappingSimpleCFF2NoPanic(t *testing.T) {
	stream := makeCFF2Stream(t, glyphdata.OpenTypeGlyf)

	enc := encoding.Simple(func(byte) string { return encoding.UseBuiltin })
	f := &mockFontInfoSimple{
		fontInfo: &dict.FontInfoSimple{
			PostScriptName: "Test",
			FontFile:       stream,
			Encoding:       enc,
			IsSymbolic:     true,
		},
	}
	got := GlyphNameMapping(f)
	if got != nil {
		t.Errorf("expected nil for a CFF2-backed glyf stream, got %v", got)
	}
}

// makeCFF2Stream builds a minimal, non-variable CFF2 sfnt font and wraps its
// raw encoding (a real OpenType/CFF2 font) in a glyphdata.Stream labeled tp,
// simulating a malformed or mislabeled embedded font file.
func makeCFF2Stream(t *testing.T, tp glyphdata.Type) *glyphdata.Stream {
	t.Helper()

	b := func(v float64) cff.Blend { return cff.Blend{Default: v} }
	box := &cff.GlyphCFF2{Cmds: []cff.GlyphOpCFF2{
		{Op: cff.OpMoveTo, Args: []cff.Blend{b(0), b(0)}},
		{Op: cff.OpLineTo, Args: []cff.Blend{b(500), b(0)}},
		{Op: cff.OpLineTo, Args: []cff.Blend{b(500), b(700)}},
		{Op: cff.OpLineTo, Args: []cff.Blend{b(0), b(700)}},
	}}
	o := &cff.OutlinesCFF2{
		Glyphs:   []*cff.GlyphCFF2{box, box},
		Widths:   []float64{600, 600},
		Private:  []*cff.PrivateCFF2{{}},
		FDSelect: func(glyph.ID) int { return 0 },
	}
	font := &sfnt.Font{
		FamilyName: "GlyphnamesCFF2Test",
		UnitsPerEm: 1000,
		Ascent:     700,
		Descent:    -300,
		CapHeight:  700,
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
		Outlines:   o,
	}

	var buf bytes.Buffer
	if _, err := font.Write(&buf); err != nil {
		t.Fatalf("write CFF2 font: %v", err)
	}
	data := buf.Bytes()

	return &glyphdata.Stream{
		Type: tp,
		WriteTo: func(w io.Writer, _ *glyphdata.Lengths) error {
			_, err := w.Write(data)
			return err
		},
	}
}

// makeCFFStream creates a minimal CFFSimple stream whose built-in
// encoding maps the given codes to the given glyph names.
func makeCFFStream(t *testing.T, codeToName map[byte]string) *glyphdata.Stream {
	t.Helper()

	nameToGID := map[string]glyph.ID{".notdef": 0}
	glyphs := []*cff.Glyph{{Name: ".notdef"}}
	for _, name := range codeToName {
		if _, ok := nameToGID[name]; ok {
			continue
		}
		nameToGID[name] = glyph.ID(len(glyphs))
		glyphs = append(glyphs, &cff.Glyph{Name: name})
	}

	enc := make([]glyph.ID, 256)
	for code, name := range codeToName {
		enc[code] = nameToGID[name]
	}

	font := &cff.Font{
		FontInfo: &type1.FontInfo{
			FontName:   "Test",
			FontMatrix: [6]float64{0.001, 0, 0, 0.001, 0, 0},
		},
		Outlines: &cff.Outlines{
			Glyphs:   glyphs,
			Private:  []*type1.PrivateDict{{}},
			FDSelect: func(glyph.ID) int { return 0 },
			Encoding: enc,
		},
	}
	return cffglyphs.ToStream(font, glyphdata.CFFSimple)
}

// mockFontInfoGlyfEmbedded wraps mockFontInstance but returns a
// FontInfoGlyfEmbedded from FontInfo.
type mockFontInfoGlyfEmbedded struct {
	mockFontInstance
	fontInfo *dict.FontInfoGlyfEmbedded
}

func (f *mockFontInfoGlyfEmbedded) FontInfo() any {
	return f.fontInfo
}

// mockFontInfoSimple wraps mockFontInstance but returns a
// FontInfoSimple from FontInfo.
type mockFontInfoSimple struct {
	mockFontInstance
	fontInfo *dict.FontInfoSimple
}

func (f *mockFontInfoSimple) FontInfo() any {
	return f.fontInfo
}
