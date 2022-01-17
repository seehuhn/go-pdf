// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package cff

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type1"
)

// Subset returns a copy of the font, including only the glyphs in the given
// subset.  The ".notdef" glyph must always be included as the first glyph.
func (cff *Font) Subset(subset []font.GlyphID) *Font {
	if subset[0] != 0 {
		panic("invalid subset")
	}

	tag := font.GetSubsetTag(subset, len(cff.GlyphNames))
	out := &Font{
		Meta: &type1.FontDict{
			FontName: pdf.Name(tag) + "+" + cff.Meta.FontName,
			// TODO(voss): copy the rest
		},
		GlyphNames:  make([]string, 0, len(subset)),
		GlyphExtent: make([]font.Rect, 0, len(subset)),
		Width:       make([]int, 0, len(subset)),

		privateDict: cff.privateDict,

		gid2cid: append([]font.GlyphID{}, subset...),
	}

	for _, gid := range subset {
		out.GlyphNames = append(out.GlyphNames, cff.GlyphNames[gid])
		out.GlyphExtent = append(out.GlyphExtent, cff.GlyphExtent[gid])
		out.Width = append(out.Width, cff.Width[gid])
	}

	charStrings := make(cffIndex, 0, len(subset))
	for _, gid := range subset {
		// expand all subroutines
		// TODO(voss): re-introduce subroutines as needed

		cmds, err := cff.doDecode(nil, cff.charStrings[gid])
		if err != nil {
			// We failed to decode a charstring, so we cannot reliably
			// prune the subroutines.  Use naive subsetting instead.
			out.gsubrs = cff.gsubrs
			out.subrs = cff.subrs

			for _, gid := range subset {
				out.charStrings = append(out.charStrings, cff.charStrings[gid])
			}
			return out
		}

		var cc []byte
		for _, cmd := range cmds {
			cc = append(cc, cmd...)
		}

		charStrings = append(charStrings, cc)
	}
	out.charStrings = charStrings

	return out
}
