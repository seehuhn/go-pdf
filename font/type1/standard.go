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

import "fmt"

// GetStandardWidth returns the width of glyphs in the 14 standard PDF fonts.
// The width is given in PDF glyph space units (i.e. are multiplied by 1000).
func GetStandardWidth(fontName, glyphName string) (float64, error) {
	m, ok := builtinMetrics[fontName]
	if !ok {
		return 0, fmt.Errorf("unknown standard font %q", fontName)
	}

	w, ok := m.Widths[glyphName]
	if !ok {
		return 0, fmt.Errorf("unknown glyph %q in font %q", glyphName, fontName)
	}

	return w, nil
}
