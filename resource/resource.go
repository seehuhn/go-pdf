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

package resource

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/property"
)

// TODO(voss):
// * Implement ExtractExtGState for [graphics.ExtGState]
// * Implement ExtractPattern for color.Pattern interface
// * Review/verify shading.Extract compatibility with the extractor pattern
// * Sort out fonts
// * Sort out Properties

// PDF 2.0 sections: 14.2 14.6 7.8

type Resource struct {
	ExtGState  map[pdf.Name]graphics.ExtGState
	ColorSpace map[pdf.Name]color.Space
	Pattern    map[pdf.Name]color.Pattern
	Shading    map[pdf.Name]graphics.Shading
	XObject    map[pdf.Name]graphics.XObject
	Font       map[pdf.Name]font.Instance
	Properties map[pdf.Name]property.List
	ProcSet    ProcSet
}

// var _ pdf.Embedder = (*Resource)(nil)

type ProcSet struct {
	PDF    bool
	Text   bool
	ImageB bool
	ImageC bool
	ImageI bool
}
