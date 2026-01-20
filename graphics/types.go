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

// XObject represents an external object—a self-contained piece of graphical
// content (image, form, etc.) that can be referenced by name and drawn
// multiple times.
//
// See [seehuhn.de/go/pdf/graphics/image] and [seehuhn.de/go/pdf/graphics/form]
// for implementations.
type XObject interface {
	Subtype() pdf.Name
	pdf.Embedder
}

// Image represents a raster image XObject with known pixel dimensions.
//
// See [seehuhn.de/go/pdf/graphics/image] for implementations.
type Image interface {
	XObject
	Bounds() rect.IntRect
}

// ImageMask is an optional interface implemented by XObjects that are
// stencil masks (PDF image XObjects with ImageMask=true).
// Image masks are allowed in contexts where other images are forbidden
// (uncolored tiling patterns and Type 3 glyphs with d1).
type ImageMask interface {
	XObject
	IsImageMask() bool
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

// Shading defines a smooth color gradient that can fill an area.
// Shadings can be drawn directly or used as the basis of a shading pattern.
//
// See [seehuhn.de/go/pdf/graphics/shading] for implementations.
type Shading interface {
	ShadingType() int
	Equal(other Shading) bool

	pdf.Embedder
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

// Halftone represents a PDF halftone dictionary or stream.
//
// See [seehuhn.de/go/pdf/graphics/halftone] for implementations.
type Halftone interface {
	// HalftoneType returns the halftone type (1, 5, 6, 10, or 16).
	HalftoneType() int

	// GetTransferFunction returns the transfer function specified in the
	// halftone, or nil if none is specified.
	GetTransferFunction() pdf.Function

	pdf.Embedder
}

// TransferFunctions holds the transfer functions for the color components.
// Transfer functions adjust color component values during rendering,
// mapping input values to output values for each color channel.
//
// Each function must have one input and one output.
// Use [function.Identity] to represent the PDF name /Identity.
// Use nil to represent the device-specific default transfer function.
type TransferFunctions struct {
	Red   pdf.Function
	Green pdf.Function
	Blue  pdf.Function
	Gray  pdf.Function
}

// BlendMode represents a PDF blend mode.
// Internally stored as a slice to handle the deprecated array form.
// When writing: len==1 emits name, len>1 emits array.
// When reading: name becomes len==1 slice, array becomes full slice.
//
// See PDF 32000-2:2020, sections 8.4.5, 11.3.5, 11.6.3.
type BlendMode []pdf.Name

// All 16 standard blend mode names (section 11.3.5).
const (
	BlendModeNormal     pdf.Name = "Normal"
	BlendModeCompatible pdf.Name = "Compatible" // deprecated in PDF 2.0
	BlendModeMultiply   pdf.Name = "Multiply"
	BlendModeScreen     pdf.Name = "Screen"
	BlendModeOverlay    pdf.Name = "Overlay"
	BlendModeDarken     pdf.Name = "Darken"
	BlendModeLighten    pdf.Name = "Lighten"
	BlendModeColorDodge pdf.Name = "ColorDodge"
	BlendModeColorBurn  pdf.Name = "ColorBurn"
	BlendModeHardLight  pdf.Name = "HardLight"
	BlendModeSoftLight  pdf.Name = "SoftLight"
	BlendModeDifference pdf.Name = "Difference"
	BlendModeExclusion  pdf.Name = "Exclusion"
	BlendModeHue        pdf.Name = "Hue"
	BlendModeSaturation pdf.Name = "Saturation"
	BlendModeColor      pdf.Name = "Color"
	BlendModeLuminosity pdf.Name = "Luminosity"
)

// AsPDF returns the PDF representation: name for single mode, array for multiple.
func (m BlendMode) AsPDF() pdf.Object {
	switch len(m) {
	case 0:
		return nil
	case 1:
		return m[0]
	default:
		arr := make(pdf.Array, len(m))
		for i, n := range m {
			arr[i] = n
		}
		return arr
	}
}

// IsZero returns true if the BlendMode is empty (unset).
func (m BlendMode) IsZero() bool {
	return len(m) == 0
}

// Equal reports whether two BlendModes are equal.
func (m BlendMode) Equal(other BlendMode) bool {
	if len(m) != len(other) {
		return false
	}
	for i, n := range m {
		if n != other[i] {
			return false
		}
	}
	return true
}

// LineCapStyle is the style of the end of a line.
type LineCapStyle uint8

// Possible values for LineCapStyle.
// See section 8.4.3.3 of PDF 32000-1:2008.
const (
	LineCapButt   LineCapStyle = 0
	LineCapRound  LineCapStyle = 1
	LineCapSquare LineCapStyle = 2
)

func (c LineCapStyle) String() string {
	switch c {
	case LineCapRound:
		return "round"
	case LineCapSquare:
		return "square"
	default:
		return "butt"
	}
}

// LineJoinStyle is the style of the corner of a line.
type LineJoinStyle uint8

// Possible values for LineJoinStyle.
// See section 8.4.3.4 of PDF 32000-1:2008.
const (
	LineJoinMiter LineJoinStyle = 0
	LineJoinRound LineJoinStyle = 1
	LineJoinBevel LineJoinStyle = 2
)

func (j LineJoinStyle) String() string {
	switch j {
	case LineJoinRound:
		return "round"
	case LineJoinBevel:
		return "bevel"
	default:
		return "miter"
	}
}

// RenderingIntent controls how colors are adjusted when converting between
// color spaces with different gamuts.
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

// TextRenderingMode is the rendering mode for text.
type TextRenderingMode uint8

// Possible values for TextRenderingMode.
// See section 9.3.6 of ISO 32000-2:2020.
const (
	TextRenderingModeFill           TextRenderingMode = 0
	TextRenderingModeStroke         TextRenderingMode = 1
	TextRenderingModeFillStroke     TextRenderingMode = 2
	TextRenderingModeInvisible      TextRenderingMode = 3
	TextRenderingModeFillClip       TextRenderingMode = 4
	TextRenderingModeStrokeClip     TextRenderingMode = 5
	TextRenderingModeFillStrokeClip TextRenderingMode = 6
	TextRenderingModeClip           TextRenderingMode = 7
)

// MarkedContent represents a marked-content point or sequence in a content
// stream, used for accessibility tagging, optional content layers, or logical
// document structure.
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
