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

package simple

import (
	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
)

var _ font.Embedded = (*Type3Dict)(nil)

// Type3Dict represents a Type 3 font dictionary.
type Type3Dict struct {
	// Ref is the reference to the font dictionary in the PDF file.
	Ref pdf.Reference

	// Name is deprecated and is normally empty (optional).
	// For PDF 1.0 this was the name the font was referenced by from
	// within content streams.
	Name pdf.Name

	FontMatrix matrix.Matrix

	// Encoding maps character codes to glyph names.
	Encoding encoding.Type1

	// Width contains the glyph widths for all character codes
	// (PDF glyph space units).
	Width [256]float64

	// Descriptor is the font descriptor (optional).
	Descriptor *font.Descriptor

	// TODO(voss): Resources

	// Text gives the text content for each character code.
	Text [256]string

	// GetFont (optional) returns the font data to embed.
	// If this is nil, the font data is not embedded in the PDF file.
	GetFont func() (Type1FontData, error)
}

func (d *Type3Dict) WritingMode() cmap.WritingMode {
	return cmap.Horizontal
}

func (d *Type3Dict) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	return d.Width[s[0]], 1
}
