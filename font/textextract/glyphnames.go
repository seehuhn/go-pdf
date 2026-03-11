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
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
)

// GlyphNameMapping returns a mapping from CID to Unicode text based on
// glyph names in the embedded font file.
// This is used as a fallback when the font's ToUnicode map has no entry
// for a CID.
// Returns nil for unsupported font types.
func GlyphNameMapping(f font.Instance) map[cid.CID]string {
	fontInfo := f.FontInfo()

	fi, ok := fontInfo.(*dict.FontInfoGlyfEmbedded)
	if !ok || fi.FontFile == nil {
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
		if name == "" {
			continue
		}
		text := names.ToUnicode(name, fi.PostScriptName)
		m[cid.CID(cidVal)] = text
	}
	return m
}
