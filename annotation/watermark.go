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

package annotation

import (
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.5.2 12.5.6.22

// Watermark represents a watermark annotation, used for graphics printed at a
// fixed size and position relative to the target media, regardless of the
// media's dimensions.
//
// Watermark annotations have no pop-up window or other interactive elements.
// When displayed on-screen, interactive PDF processors use the media box as the
// media dimensions.
type Watermark struct {
	Common

	// FixedPrint (optional) is a fixed print dictionary that specifies how
	// the annotation is drawn relative to the dimensions of the target media.
	// If this entry is not present, the annotation is drawn without any
	// special consideration for the dimensions of the target media.
	FixedPrint *FixedPrint
}

// FixedPrint represents a fixed print dictionary that specifies how a
// watermark annotation is drawn relative to the dimensions of the target
// media.
//
// With a fixed print dictionary, the watermark is drawn in media space, with
// the page's own scale and rotation cancelled. As a result, the Matrix and the
// width and height of the annotation's Rect are fixed physical measures in
// units of UserUnit/72 inch, while the position of Rect is discarded; the box
// is placed by anchoring it at the fraction (H, V) of the target media's width
// and height and then offsetting or rotating it by Matrix.
type FixedPrint struct {
	// Matrix transforms the annotation's rectangle before rendering.
	// When positioning content near the edge of the media, this can be used to
	// provide a reasonable offset to allow for unprintable margins.
	//
	// On write, a zero Matrix is treated as a shorthand for the identity matrix.
	Matrix matrix.Matrix

	// H is the amount to translate the associated content horizontally, as a
	// fraction of the width of the target media (or, if unknown, the width of
	// the page's MediaBox). A value of 1.0 represents the full width.
	H float64

	// V is the amount to translate the associated content vertically, as a
	// fraction of the height of the target media (or, if unknown, the height
	// of the page's MediaBox). A value of 1.0 represents the full height.
	V float64

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ Annotation = (*Watermark)(nil)
var _ pdf.Embedder = (*FixedPrint)(nil)

// AnnotationType returns "Watermark".
// This implements the [Annotation] interface.
func (w *Watermark) AnnotationType() pdf.Name {
	return "Watermark"
}

func (w *Watermark) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "watermark annotation", pdf.V1_6); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Watermark"),
	}

	// Add common annotation fields
	if err := w.Common.fillDict(rm, dict, isMarkup(w), false); err != nil {
		return nil, err
	}

	// FixedPrint (optional)
	if w.FixedPrint != nil {
		fp, err := rm.Embed(w.FixedPrint)
		if err != nil {
			return nil, err
		}
		dict["FixedPrint"] = fp
	}

	return dict, nil
}

// ExtractFixedPrint reads a fixed print dictionary from a PDF file.
func ExtractFixedPrint(c pdf.Cursor, obj pdf.Object, isDirect bool) (*FixedPrint, error) {
	dict, err := c.DictTyped(obj, "FixedPrint")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing fixed print dictionary")
	}

	fp := &FixedPrint{SingleUse: isDirect}

	if m, err := pdf.Optional(c.Matrix(dict["Matrix"])); err != nil {
		return nil, err
	} else if m != matrix.Zero {
		fp.Matrix = m
	} else {
		fp.Matrix = matrix.Identity
	}

	if h, err := pdf.Optional(c.Number(dict["H"])); err != nil {
		return nil, err
	} else {
		fp.H = h
	}

	if v, err := pdf.Optional(c.Number(dict["V"])); err != nil {
		return nil, err
	} else {
		fp.V = v
	}

	return fp, nil
}

// Embed adds the fixed print dictionary to the PDF file.
// This implements the [pdf.Embedder] interface.
func (fp *FixedPrint) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "fixed print dictionary", pdf.V1_6); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Type": pdf.Name("FixedPrint"),
	}

	if fp.Matrix != matrix.Identity && fp.Matrix != matrix.Zero {
		m := fp.Matrix
		dict["Matrix"] = pdf.Array{
			pdf.Number(m[0]), pdf.Number(m[1]), pdf.Number(m[2]),
			pdf.Number(m[3]), pdf.Number(m[4]), pdf.Number(m[5]),
		}
	}
	if fp.H != 0 {
		dict["H"] = pdf.Number(fp.H)
	}
	if fp.V != 0 {
		dict["V"] = pdf.Number(fp.V)
	}

	if fp.SingleUse {
		return dict, nil
	}
	ref := rm.Alloc()
	if err := rm.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}
