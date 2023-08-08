// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package type3

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/sfnt/glyph"
)

type embedded struct {
	w pdf.Putter
	*Font
	pdf.Resource
	cmap.SimpleEncoder
}

func (e *embedded) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	return append(s, e.Encode(gid, rr))
}

func (e *embedded) Close() error {
	encoding := e.Encoding()

	subset := make(map[pdf.Name]*Glyph)
	for _, gid := range encoding {
		// Gid 0 maps to the empty glyph name, which is not in the charProcs map.
		if glyph := e.charProcs[e.glyphNames[gid]]; glyph != nil {
			subset[e.glyphNames[gid]] = glyph
		}
	}

	var descriptor *font.Descriptor
	if pdf.IsTagged(e.w) {
		descriptor = &font.Descriptor{
			IsFixedPitch: e.IsFixedPitch,
			IsSerif:      e.IsSerif,
			IsScript:     e.IsScript,
			IsItalic:     e.IsItalic,
			IsAllCap:     e.IsAllCap,
			IsSmallCap:   e.IsSmallCap,
			ForceBold:    e.ForceBold,
			ItalicAngle:  e.ItalicAngle,
		}
		if pdf.GetVersion(e.w) == pdf.V1_0 {
			// required by the amended PDF 2.0 specification
			descriptor.FontName = e.Name
		}
	}

	info := &PDFFont{
		FontMatrix: e.fontMatrix,
		Glyphs:     subset,
		Resources:  e.resources,
		Descriptor: descriptor,
		Encoding:   []string{},
		ToUnicode:  map[charcode.CharCode][]rune{},
		ResName:    e.Name,
	}
	return info.Embed(e.w, e.Ref)
}
