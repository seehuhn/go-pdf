// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package dict

import (
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

// moreThanTen returns true if the flattened array has more than 10 elements.
func moreThanTen(a pdf.Array) bool {
	count := 0
	for _, obj := range a {
		if a, ok := obj.(pdf.Array); ok {
			count += len(a)
		} else {
			count++
		}
		if count > 10 {
			return true
		}
	}
	return false
}

// widthsAreCompatible returns true, if the glyph widths ww are compatible with
// the standard font metrics.  The object encObj is the value of the font
// dictionary's Encoding entry.
func widthsAreCompatible(ww []float64, enc encoding.Type1, info *stdmtx.FontData) bool {
	for code := range 256 {
		glyphName := enc(byte(code))
		if glyphName == "" {
			continue
		}
		if math.Abs(ww[code]-info.Width[glyphName]) > 0.5 {
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
	if fd.IsSymbolic != stdInfo.IsSymbolic {
		return false
	}
	if fd.IsScript || fd.IsItalic || fd.IsAllCap || fd.IsSmallCap || fd.ForceBold {
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
