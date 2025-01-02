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

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/encoding"
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
		if glyphName == ".notdef" || glyphName == notdefForce {
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

// widthsAreCompatible returns true, if the glyph widths ww are compatible with
// the standard font metrics.  The object encObj is the value of the font
// dictionary's Encoding entry.
//
// EncObj must be valid and must be a direct object.  Do not pass encObj values
// read from files without validation.
func widthsAreCompatible(ww []float64, enc encoding.Type1, info *stdmtx.FontData) bool {
	for code := range 256 {
		name := enc(byte(code))
		if name == "" {
			continue
		}
		if math.Abs(ww[code]-info.Width[name]) > 0.5 {
			return false
		}
	}
	return true
}

func fontDescriptorIsCompatible(fd *font.Descriptor, stdInfo *stdmtx.FontData) bool {
	if fd.FontFamily != "" && fd.FontFamily != stdInfo.FontFamily {
		return false
	}
	if fd.FontWeight != 0 && fd.FontWeight != stdInfo.FontWeight {
		return false
	}
	if fd.IsFixedPitch != stdInfo.IsFixedPitch {
		return false
	}
	if fd.IsSerif != stdInfo.IsSerif {
		return false
	}
	if math.Abs(fd.ItalicAngle-stdInfo.ItalicAngle) > 0.1 {
		return false
	}
	if fd.Ascent != 0 && math.Abs(fd.Ascent-stdInfo.Ascent) > 0.5 {
		return false
	}
	if fd.Descent != 0 && math.Abs(fd.Descent-stdInfo.Descent) > 0.5 {
		return false
	}
	if fd.CapHeight != 0 && math.Abs(fd.CapHeight-stdInfo.CapHeight) > 0.5 {
		return false
	}
	if fd.XHeight != 0 && math.Abs(fd.XHeight-stdInfo.XHeight) > 0.5 {
		return false
	}
	if fd.StemV != 0 && math.Abs(fd.StemV-stdInfo.StemV) > 0.5 {
		return false
	}
	if fd.StemH != 0 && math.Abs(fd.StemH-stdInfo.StemH) > 0.5 {
		return false
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
