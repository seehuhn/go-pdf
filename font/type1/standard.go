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
	"fmt"
	"math"

	"seehuhn.de/go/pdf/internal/stdmtx"
)

// IsStandard returns true if the font is one of the standard 14 PDF fonts.
// This is determined by the font name, the set of glyphs used, and the glyph
// widths.
//
// ww must be the widths of the 256 encoded characters, given in PDF text space
// units times 1000.
func isStandard(fontName string, enc []string, ww []float64) bool {
	m, ok := stdmtx.Metrics[fontName]
	if !ok {
		return false
	}

	for i, glyphName := range enc {
		if glyphName == "" || glyphName == ".notdef" || glyphName == notdefForce {
			continue
		}
		w, ok := m.Width[glyphName]
		if !ok {
			return false
		}
		if math.Abs(ww[i]-w) > 0.5 {
			return false
		}
	}
	return true
}

// GetStandardWidth returns the width of glyphs in the 14 standard PDF fonts.
// The width is given in PDF glyph space units (i.e. are multiplied by 1000).
//
// TODO(voss): remove
func GetStandardWidth(fontName, glyphName string) (float64, error) {
	m, ok := stdmtx.Metrics[fontName]
	if !ok {
		return 0, fmt.Errorf("unknown standard font %q", fontName)
	}

	w, ok := m.Width[glyphName]
	if !ok {
		return 0, fmt.Errorf("unknown glyph %q in font %q", glyphName, fontName)
	}

	return w, nil
}
