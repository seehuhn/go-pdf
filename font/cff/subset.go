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

// Subset returns a copy of the font, including only the glyphs in the given
// subset.  The ".notdef" glyph is always included as the first glyph.
func (cff *Font) Subset(subset []font.GlyphID) *Font {
	out := &Font{
		FontName:    cff.FontName, // TODO(voss): subset tag needed?
		topDict:     cff.topDict,
		privateDict: cff.privateDict,

		gid2cid: append([]font.GlyphID{}, subset...),
	}

	// TODO(voss): re-introduce subroutines as needed

	out.charStrings = cffIndex{cff.charStrings[0]}
	out.GlyphName = []string{cff.GlyphName[0]}
	for _, gid := range subset {
		if gid != 0 {
			// expand all subroutines
			cmds, err := cff.decodeCharString(cff.charStrings[gid], nil)
			if err != nil {
				// We failed to decode charstring, so we cannot reliably
				// prune the subroutines.  Use naive subsetting instead.
				return cff.naiveSubset(subset)
			}
			var cc []byte
			for _, cmd := range cmds {
				cc = append(cc, cmd...)
			}

			out.charStrings = append(out.charStrings, cc)
			out.GlyphName = append(out.GlyphName, cff.GlyphName[gid])
		}
	}

	return out
}

// naiveSubset returns a copy of the font with only the glyphs in the given
// subset.  The ".notdef" glyph is always included as the first glyph.
// This method does not prune/expand subroutines.
func (cff *Font) naiveSubset(subset []font.GlyphID) *Font {
	out := &Font{
		FontName:    cff.FontName, // TODO(voss): subset tag needed?
		topDict:     cff.topDict,
		privateDict: cff.privateDict,
		gsubrs:      cff.gsubrs,
		subrs:       cff.subrs,

		gid2cid: append([]font.GlyphID{}, subset...),
	}

	out.charStrings = cffIndex{cff.charStrings[0]}
	out.GlyphName = []string{cff.GlyphName[0]}
	for _, gid := range subset {
		out.charStrings = append(out.charStrings, cff.charStrings[gid])
		out.GlyphName = append(out.GlyphName, cff.GlyphName[gid])
	}

	return out
}
