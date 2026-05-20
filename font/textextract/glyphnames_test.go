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
	"testing"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1"
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
