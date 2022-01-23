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
)

// Subset returns a copy of the font, including only the glyphs in the given
// subset.  The ".notdef" glyph must always be included as the first glyph.
func (cff *Font) Subset(subset []font.GlyphID) *Font {
	if subset[0] != 0 {
		panic("invalid subset")
	}

	tag := font.GetSubsetTag(subset, len(cff.Glyphs))
	info := *cff.Info
	info.FontName = pdf.Name(tag) + "+" + cff.Info.FontName
	out := &Font{
		Info: &info,
	}

	for _, gid := range subset {
		out.Glyphs = append(out.Glyphs, cff.Glyphs[gid])
	}

	return out
}
