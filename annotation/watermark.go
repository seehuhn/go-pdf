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

import "seehuhn.de/go/pdf"

// FixedPrint represents a fixed print dictionary that specifies how a watermark
// annotation is drawn relative to the dimensions of the target media.
type FixedPrint struct {
	// Matrix (optional) is the matrix used to transform the annotation's
	// rectangle before rendering. Default value: the identity matrix [1 0 0 1 0 0].
	// When positioning content near the edge of the media, this entry should be
	// used to provide a reasonable offset to allow for unprintable margins.
	Matrix []float64

	// H (optional) is the amount to translate the associated content horizontally,
	// as a percentage of the width of the target media (or if unknown, the width
	// of the page's MediaBox). 1.0 represents 100% and 0.0 represents 0%.
	// Negative values should not be used. Default value: 0.
	H float64

	// V (optional) is the amount to translate the associated content vertically,
	// as a percentage of the height of the target media (or if unknown, the height
	// of the page's MediaBox). 1.0 represents 100% and 0.0 represents 0%.
	// Negative values should not be used. Default value: 0.
	V float64
}

// Watermark represents a watermark annotation used to represent graphics that
// are to be printed at a fixed size relative to the target media, and fixed
// relative position on the target media, regardless of the dimensions of that media.
//
// Watermark annotations have no popup window nor other interactive elements.
// When displaying on-screen, interactive PDF processors use the dimensions
// of the media box as the media dimensions so that scroll and zoom behavior is
// the same as for other annotations.
type Watermark struct {
	Common

	// FixedPrint (optional) is a fixed print dictionary that specifies how this
	// annotation is drawn relative to the dimensions of the target media.
	// If this entry is not present, the annotation is drawn without any
	// special consideration for the dimensions of the target media.
	FixedPrint *FixedPrint
}

var _ Annotation = (*Watermark)(nil)

// AnnotationType returns "Watermark".
// This implements the [Annotation] interface.
func (w *Watermark) AnnotationType() pdf.Name {
	return "Watermark"
}

func decodeWatermark(r pdf.Getter, dict pdf.Dict) (*Watermark, error) {
	watermark := &Watermark{}

	// Extract common annotation fields
	if err := decodeCommon(r, &watermark.Common, dict); err != nil {
		return nil, err
	}

	// Extract watermark-specific fields
	// FixedPrint (optional)
	if fixedPrintRef, ok := dict["FixedPrint"].(pdf.Reference); ok {
		fixedPrintDict, err := pdf.GetDictTyped(r, fixedPrintRef, "FixedPrint")
		if err != nil {
			return nil, err
		}

		fixedPrint := &FixedPrint{}

		// Matrix (optional) - default to identity matrix
		if matrix, err := pdf.GetArray(r, fixedPrintDict["Matrix"]); err == nil && len(matrix) == 6 {
			matrixValues := make([]float64, 6)
			for i, val := range matrix {
				if num, err := pdf.GetNumber(r, val); err == nil {
					matrixValues[i] = float64(num)
				}
			}
			fixedPrint.Matrix = matrixValues
		} else {
			// Default identity matrix
			fixedPrint.Matrix = []float64{1, 0, 0, 1, 0, 0}
		}

		// H (optional) - default 0
		if h, err := pdf.GetNumber(r, fixedPrintDict["H"]); err == nil {
			fixedPrint.H = float64(h)
		}

		// V (optional) - default 0
		if v, err := pdf.GetNumber(r, fixedPrintDict["V"]); err == nil {
			fixedPrint.V = float64(v)
		}

		watermark.FixedPrint = fixedPrint
	}

	return watermark, nil
}

func (w *Watermark) Encode(rm *pdf.ResourceManager) (pdf.Dict, error) {
	if err := pdf.CheckVersion(rm.Out, "watermark annotation", pdf.V1_6); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Watermark"),
	}

	// Add common annotation fields
	if err := w.Common.fillDict(rm, dict, isMarkup(w)); err != nil {
		return nil, err
	}

	// Add watermark-specific fields
	// FixedPrint (optional)
	if w.FixedPrint != nil {
		fixedPrintDict := pdf.Dict{
			"Type": pdf.Name("FixedPrint"),
		}

		// Matrix (optional) - only write if not identity matrix
		isIdentityMatrix := len(w.FixedPrint.Matrix) == 6 &&
			w.FixedPrint.Matrix[0] == 1 && w.FixedPrint.Matrix[1] == 0 &&
			w.FixedPrint.Matrix[2] == 0 && w.FixedPrint.Matrix[3] == 1 &&
			w.FixedPrint.Matrix[4] == 0 && w.FixedPrint.Matrix[5] == 0

		if !isIdentityMatrix && len(w.FixedPrint.Matrix) == 6 {
			matrixArray := make(pdf.Array, 6)
			for i, val := range w.FixedPrint.Matrix {
				matrixArray[i] = pdf.Number(val)
			}
			fixedPrintDict["Matrix"] = matrixArray
		}

		// H (optional) - only write if not default 0
		if w.FixedPrint.H != 0 {
			fixedPrintDict["H"] = pdf.Number(w.FixedPrint.H)
		}

		// V (optional) - only write if not default 0
		if w.FixedPrint.V != 0 {
			fixedPrintDict["V"] = pdf.Number(w.FixedPrint.V)
		}

		// Embed the FixedPrint dictionary
		fixedPrintRef := rm.Out.Alloc()
		rm.Out.Put(fixedPrintRef, fixedPrintDict)
		dict["FixedPrint"] = fixedPrintRef
	}

	return dict, nil
}
