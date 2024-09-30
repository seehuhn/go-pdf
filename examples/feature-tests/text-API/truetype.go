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
	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/graphics"
)

func trueTypeRune(w graphics.Writer, f *sfnt.Font, r rune) {
	cmap, err := f.CMapTable.GetBest()
	if err != nil {
		panic(err)
	}
	enc := encoding.New()

	// -----------------------------------------------------------------------

	gid := cmap.Lookup(r)
	if gid == 0 {
		panic("missing rune")
	}
	text := string([]rune{r})

	code, isNew := allocateCode(gid, text)
	if isNew {
		// builtinEncoding[code] = gid

		cid := enc.UseBuiltinEncoding(code[0])
		w := f.GlyphWidthPDF(gid)

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
