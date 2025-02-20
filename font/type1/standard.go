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

package type1

import (
	"math"

	"seehuhn.de/go/pdf/font/encoding/simpleenc"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

// IsStandard returns true if the font is one of the standard 14 PDF fonts.
// This is determined by the font name, the set of glyphs used, and the glyph
// widths.
//
// ww must be the widths of the 256 encoded characters, given in PDF text space
// units times 1000.
func isStandard(fontName string, t *simpleenc.Simple) bool {
	m, ok := stdmtx.Metrics[fontName]
	if !ok {
		return false
	}

	for code, info := range t.MappedCodes() {
		gid := t.GID(byte(code))
		glyphName := t.GlyphName(gid)
		wStd, ok := m.Width[glyphName]
		if !ok {
			// The glyph is not in the standard font.
			return false
		}

		wOurs := info.Width

		if math.Abs(wStd-wOurs) > 0.5 {
			return false
		}
	}
	return true
}
