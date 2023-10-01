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

package font

import (
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/sfnt"
)

// IsStandardLatin returns true if all glyphs are in the Adobe Standard Latin
// character set.
//
// TODO(voss): move this somewhere else.
func IsStandardLatin(f *sfnt.Font) bool {
	glyphNames := f.MakeGlyphNames()
	for _, name := range glyphNames {
		if !pdfenc.IsStandardLatin[name] {
			return false
		}
	}
	return true
}
