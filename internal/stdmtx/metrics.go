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

package stdmtx

import (
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf/font/pdfenc"

	"seehuhn.de/go/sfnt/os2"
)

// FontData contains metrics for one of the 14 standard fonts.
// All lengths are in PDF glyph space units.
type FontData struct {
	FontFamily   string
	FontWeight   os2.Weight
	IsFixedPitch bool
	IsSerif      bool
	IsSymbolic   bool
	FontBBox     rect.Rect
	ItalicAngle  float64
	Ascent       float64
	Descent      float64 // negative
	CapHeight    float64
	XHeight      float64
	StemV        float64
	StemH        float64

	// Width contains the width of each glyph, in PDF glyph space units.
	Width map[string]float64

	Encoding []string
}

var Metrics map[string]*FontData = metrics

var (
	standardEncoding     = pdfenc.Standard.Encoding[:]
	symbolEncoding       = pdfenc.Symbol.Encoding[:]
	zapfDingbatsEncoding = pdfenc.ZapfDingbats.Encoding[:]
)
