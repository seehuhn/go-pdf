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

import "seehuhn.de/go/pdf/font"

// Subset returns a copy of the font with only the glyphs in the given
// subset.  The ".notdef" glyph is always included as the first glyph.
func (cff *Font) Subset(subset []font.GlyphID) *Font {
	out := &Font{
		FontName:    cff.FontName, // TODO(voss): subset tag needed?
		topDict:     cff.topDict,
		gsubrs:      cff.gsubrs,
		privateDict: cff.privateDict,
		subrs:       cff.subrs,

		gid2cid: append([]font.GlyphID{}, subset...),
	}

	// TODO(voss): prune unused subrs

	out.charStrings = cffIndex{cff.charStrings[0]}
	out.GlyphName = []string{cff.GlyphName[0]}
	for _, gid := range subset {
		if gid != 0 {
			out.charStrings = append(out.charStrings, cff.charStrings[gid])
			out.GlyphName = append(out.GlyphName, cff.GlyphName[gid])
		}
	}

	return out
}
