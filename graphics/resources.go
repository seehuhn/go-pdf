// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package graphics

import (
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
)

// Shading represents a PDF shading dictionary.
//
// Shadings can either be drawn to the page using the [Writer.DrawShading]
// method, or can be used as the basis of a shading pattern.
type Shading interface {
	ShadingType() int

	pdf.Embedder
}

// XObject represents a PDF XObject.
type XObject interface {
	Subtype() pdf.Name
	pdf.Embedder
}

// ImageMask is an optional interface implemented by XObjects that are
// stencil masks (PDF image XObjects with ImageMask=true).
// Image masks are allowed in contexts where other images are forbidden
// (uncolored tiling patterns and Type 3 glyphs with d1).
type ImageMask interface {
	XObject
	IsImageMask() bool
}

// Image represents a raster image which can be embedded in a PDF file.
type Image interface {
	XObject
	Bounds() rect.IntRect
}

// SoftClip represents a soft mask for use in the graphics state.
//
// Soft masks define position-dependent mask values derived from a transparency
// group. Two derivation methods are supported: Alpha (using the group's
// computed alpha, ignoring color) and Luminosity (converting the group's
// computed color to a single-component luminosity value).
//
// See PDF 32000-1:2008, 11.6.5 "Specifying soft masks".
type SoftClip interface {
	pdf.Embedder
	IsSoftClip() bool
	Equals(other SoftClip) bool
}
