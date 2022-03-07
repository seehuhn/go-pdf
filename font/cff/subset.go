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
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type1"
)

// Subset returns a copy of the font, including only the glyphs in the given
// subset.  The ".notdef" glyph must always be included as the first glyph.
func (cff *Font) Subset(subset []font.GlyphID) (*Font, error) {
	if subset[0] != 0 {
		return nil, errors.New("cff:invalid subset")
	}

	if len(cff.Info.FontName) >= 7 && cff.Info.FontName[6] == '+' {
		return nil, errors.New("cff: cannot subset a subset")
	}

	fdSelect := cff.FdSelect
	if fdSelect == nil {
		fdSelect = func(gi font.GlyphID) int { return 0 }
	}

	tag := font.GetSubsetTag(subset, len(cff.Glyphs))
	info := *cff.Info
	info.FontName = pdf.Name(tag) + "+" + cff.Info.FontName
	out := &Font{
		Info:      &info,
		IsCIDFont: true,
		FdSelect:  fdSelect,
	}
	if cff.IsCIDFont {
		out.ROS = cff.ROS
	} else {
		out.ROS = &type1.ROS{
			Registry:   "Adobe",
			Ordering:   "Identity",
			Supplement: 0,
		}
	}

	for _, gid := range subset {
		out.Glyphs = append(out.Glyphs, cff.Glyphs[gid])
		out.Gid2cid = append(out.Gid2cid, int32(gid))
	}

	return out, nil
}
