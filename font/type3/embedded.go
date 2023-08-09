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
)

type embedded struct {
	w pdf.Putter
	*Font
	pdf.Resource
	cmap.SimpleEncoder
}

func (e *embedded) Close() error {
	encoding := e.Encoding()
	encodingNames := make([]string, 256)

	subset := make(map[string]*Glyph)
	for i, gid := range encoding {
		// Gid 0 maps to the empty glyph name, which is not in the charProcs map.
		if glyph := e.charProcs[e.glyphNames[gid]]; glyph != nil {
			name := e.glyphNames[gid]
			encodingNames[i] = name
			subset[name] = glyph
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
			// required by PDF 2.0 specification errata:
			// https://pdf-issues.pdfa.org/32000-2-2020/clause09.html#H9.8.1
			descriptor.FontName = e.Name
		}
	}

	var toUnicode map[charcode.CharCode][]rune
	// TODO(voss): construct a toUnicode map, when needed

	info := &EmbedInfo{
		FontMatrix: e.fontMatrix,
		Glyphs:     subset,
		Resources:  e.resources,
		Descriptor: descriptor,
		Encoding:   encodingNames,
		ToUnicode:  toUnicode,
		ResName:    e.Name,
	}
	return info.Embed(e.w, e.Ref)
}
