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
	"seehuhn.de/go/pdf/property"
)

// Shading represents a PDF shading dictionary.
//
// Shadings can be drawn directly using the DrawShading method,
// or can be used as the basis of a shading pattern.
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
	Equal(other SoftClip) bool
}

// IsImageMask returns true if the given XObject is an image mask.
func IsImageMask(xobj XObject) bool {
	if xobj.Subtype() != "Image" {
		return false
	}
	if im, ok := xobj.(ImageMask); ok {
		return im.IsImageMask()
	}
	return false
}

// MarkedContent represents a marked-content point or sequence.
type MarkedContent struct {
	// Tag specifies the role or significance of the point/sequence.
	Tag pdf.Name

	// Properties is an optional property list providing additional data.
	// Set to nil for marked content without properties (MP/BMC operators).
	Properties property.List

	// Inline controls whether the property list is embedded inline in the
	// content stream (true) or referenced via the Properties resource
	// dictionary (false). Only relevant if Properties is not nil.
	// Property lists can only be inlined if Properties.IsDirect() returns true.
	Inline bool
}

// TextRenderingMode is the rendering mode for text.
type TextRenderingMode uint8

// Possible values for TextRenderingMode.
// See section 9.3.6 of ISO 32000-2:2020.
const (
	TextRenderingModeFill TextRenderingMode = iota
	TextRenderingModeStroke
	TextRenderingModeFillStroke
	TextRenderingModeInvisible
	TextRenderingModeFillClip
	TextRenderingModeStrokeClip
	TextRenderingModeFillStrokeClip
	TextRenderingModeClip
)

// LineCapStyle is the style of the end of a line.
type LineCapStyle uint8

// Possible values for LineCapStyle.
// See section 8.4.3.3 of PDF 32000-1:2008.
const (
	LineCapButt   LineCapStyle = 0
	LineCapRound  LineCapStyle = 1
	LineCapSquare LineCapStyle = 2
)

// LineJoinStyle is the style of the corner of a line.
type LineJoinStyle uint8

// Possible values for LineJoinStyle.
// See section 8.4.3.4 of PDF 32000-1:2008.
const (
	LineJoinMiter LineJoinStyle = 0
	LineJoinRound LineJoinStyle = 1
	LineJoinBevel LineJoinStyle = 2
)

// A RenderingIntent specifies the PDF rendering intent.
//
// See section 8.6.5.8 of ISO 32000-2:2020.
type RenderingIntent pdf.Name

// The PDF standard rendering intents.
const (
	AbsoluteColorimetric RenderingIntent = "AbsoluteColorimetric"
	RelativeColorimetric RenderingIntent = "RelativeColorimetric"
	Saturation           RenderingIntent = "Saturation"
	Perceptual           RenderingIntent = "Perceptual"
)
