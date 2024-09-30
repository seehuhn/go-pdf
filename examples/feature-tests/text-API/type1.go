// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package main

import (
	"slices"

	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/graphics"
)

func type1Rune(w *graphics.Writer, f *type1.Font, r rune) {
	cmap := make(map[rune]string)
	for glyphName := range f.Glyphs {
		rr := names.ToUnicode(glyphName, f.FontName == "ZapfDingbats")
		if len(rr) != 1 {
			panic("unexpected number of runes")
		}
		cmap[rr[0]] = glyphName
	}
	enc := encoding.New()

	// -----------------------------------------------------------------------

	glyphName, ok := cmap[r]
	if !ok {
		panic("missing rune")
	}
	gidInt := slices.Index(f.GlyphList(), glyphName)
	if gidInt < 0 {
		panic("missing")
	}
	gid := glyph.ID(gidInt)
	text := string([]rune{r})

	code, isNew := allocateCode(gid, text)
	if isNew {
		// builtinEncoding[code] = glyphName

		cid := enc.Allocate(glyphName)
		w := f.Glyphs[glyphName].WidthX

		info := &font.CodeInfo{
			CID:    cid,
			Notdef: 0,
			Text:   string([]rune{r}),
			W:      w,
		}
		setCodeInfo(code, info)
	}

	w.TextShowRaw(code)
}
