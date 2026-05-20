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

// Package textextract provides shared text extraction utilities
// for mapping character identifiers to Unicode text and estimating
// space widths.
package textextract

import (
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt/glyf"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
	"seehuhn.de/go/pdf/font/glyphdata/type1glyphs"
)

// GlyphNameMapping returns a mapping from CID to Unicode text based on
// glyph names in the embedded font file.
// This is used as a fallback when the font's ToUnicode map has no entry
// for a CID.
// Returns nil for unsupported font types.
func GlyphNameMapping(f font.Instance) map[cid.CID]string {
	switch fi := f.FontInfo().(type) {
	case *dict.FontInfoGlyfEmbedded:
		return glyphNameMappingGlyfEmbedded(fi)
	case *dict.FontInfoSimple:
		return glyphNameMappingSimple(fi)
	default:
		return nil
	}
}

func glyphNameMappingGlyfEmbedded(fi *dict.FontInfoGlyfEmbedded) map[cid.CID]string {
	if fi.FontFile == nil {
		return nil
	}

	info, err := sfntglyphs.FromStream(fi.FontFile)
	if err != nil {
		return nil
	}
	outlines, ok := info.Outlines.(*glyf.Outlines)
	if !ok {
		return nil
	}

	if outlines.Names == nil || fi.CIDToGID == nil {
		return nil
	}

	m := make(map[cid.CID]string)
	for cidVal, gid := range fi.CIDToGID {
		if int(gid) >= len(outlines.Names) {
			continue
		}
		name := outlines.Names[gid]
		if name == "" || name == ".notdef" {
			continue
		}
		text := names.ToUnicode(name, fi.PostScriptName)
		if text == "" {
			continue
		}
		m[cid.CID(cidVal)] = text
	}
	return m
}

// glyphNameMappingSimple resolves character codes through the embedded
// font's built-in encoding for simple fonts (Type 1, simple CFF, or a
// simple TrueType-style font with glyph outlines).  Codes for which
// fi.Encoding returns [encoding.UseBuiltin] are resolved via the font
// itself: through the built-in encoding for Type 1 / CFF, and via the
// cmap/post selection of PDF 9.6.5.4 for TrueType / OpenType-glyf.
// For simple fonts, CIDs are character codes plus one.
func glyphNameMappingSimple(fi *dict.FontInfoSimple) map[cid.CID]string {
	if fi.FontFile == nil || fi.Encoding == nil {
		return nil
	}

	var builtin []string
	switch fi.FontFile.Type {
	case glyphdata.CFFSimple, glyphdata.OpenTypeCFFSimple:
		cffFont, err := cffglyphs.FromStream(fi.FontFile)
		if err != nil {
			return nil
		}
		builtin = cffFont.Outlines.BuiltinEncoding()
	case glyphdata.Type1:
		t1Font, err := type1glyphs.FromStream(fi.FontFile)
		if err != nil {
			return nil
		}
		builtin = t1Font.Outlines.BuiltinEncoding()
	case glyphdata.TrueType, glyphdata.OpenTypeGlyf:
		sfntFont, err := sfntglyphs.FromStream(fi.FontFile)
		if err != nil {
			return nil
		}
		outlines, ok := sfntFont.Outlines.(*glyf.Outlines)
		if !ok || len(outlines.Names) == 0 {
			return nil
		}
		sel := sfntglyphs.NewTrueTypeSelector(sfntFont, fi.IsSymbolic, fi.Encoding)
		builtin = make([]string, 256)
		for code := range 256 {
			gid, ok := sel(cid.CID(code) + 1)
			if !ok || int(gid) >= len(outlines.Names) {
				continue
			}
			builtin[code] = outlines.Names[gid]
		}
	default:
		return nil
	}

	m := make(map[cid.CID]string)
	for code := range 256 {
		name := fi.Encoding(byte(code))
		if name == "" {
			continue
		}
		if name == encoding.UseBuiltin {
			if code >= len(builtin) {
				continue
			}
			name = builtin[code]
		}
		if name == "" || name == ".notdef" {
			continue
		}
		text := names.ToUnicode(name, fi.PostScriptName)
		if text == "" {
			continue
		}
		m[cid.CID(code)+1] = text
	}
	return m
}
