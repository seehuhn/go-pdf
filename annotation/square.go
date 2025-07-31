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

// Square represents a square annotation that displays a rectangle on the page.
// When opened, it displays a popup window containing the text of the associated note.
// The rectangle is inscribed within the annotation rectangle defined by the Rect entry.
type Square struct {
	Common
	Markup

	// BorderStyle (optional) is a border style dictionary specifying the line
	// width and dash pattern that is used in drawing the rectangle.
	//
	// If this field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle

	// IC (optional; PDF 1.4) is an array of numbers in the range 0.0 to 1.0
	// specifying the interior colour with which to fill the annotation's rectangle.
	// The number of array elements determines the colour space:
	// 0 - No colour; transparent
	// 1 - DeviceGray
	// 3 - DeviceRGB
	// 4 - DeviceCMYK
	IC []float64

	// BE (optional; PDF 1.5) is a border effect dictionary describing an
	// effect applied to the border described by the BS entry.
	BE pdf.Reference

	// RD (optional; PDF 1.5) describes the numerical differences between
	// the Rect entry of the annotation and the actual boundaries of the
	// underlying square. The four numbers correspond to the differences
	// in default user space between the left, top, right, and bottom
	// coordinates of Rect and those of the square, respectively.
	RD []float64
}

var _ Annotation = (*Square)(nil)

// AnnotationType returns "Square".
// This implements the [Annotation] interface.
func (s *Square) AnnotationType() pdf.Name {
	return "Square"
}

func extractSquare(r pdf.Getter, dict pdf.Dict) (*Square, error) {
	square := &Square{}

	// Extract common annotation fields
	if err := decodeCommon(r, &square.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(r, dict, &square.Markup); err != nil {
		return nil, err
	}

	// Extract square-specific fields
	// BS (optional)
	if bs, err := pdf.Optional(ExtractBorderStyle(r, dict["BS"])); err != nil {
		return nil, err
	} else {
		square.BorderStyle = bs
	}

	// IC (optional)
	if ic, err := pdf.GetArray(r, dict["IC"]); err == nil && len(ic) > 0 {
		colors := make([]float64, len(ic))
		for i, color := range ic {
			if num, err := pdf.GetNumber(r, color); err == nil {
				colors[i] = float64(num)
			}
		}
		square.IC = colors
	}

	// BE (optional)
	if be, ok := dict["BE"].(pdf.Reference); ok {
		square.BE = be
	}

	// RD (optional)
	if rd, err := pdf.GetArray(r, dict["RD"]); err == nil && len(rd) == 4 {
		diffs := make([]float64, 4)
		for i, diff := range rd {
			if num, err := pdf.GetNumber(r, diff); err == nil {
				diffs[i] = float64(num)
			}
		}
		square.RD = diffs
	}

	return square, nil
}

func (s *Square) Encode(rm *pdf.ResourceManager) (pdf.Dict, error) {
	dict := pdf.Dict{
		"Subtype": pdf.Name("Square"),
	}

	// Add common annotation fields
	if err := s.Common.fillDict(rm, dict, isMarkup(s)); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := s.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add square-specific fields
	// BS (optional)
	if s.BorderStyle != nil {
		bs, _, err := s.BorderStyle.Embed(rm)
		if err != nil {
			return nil, err
		}
		dict["BS"] = bs
	}

	// IC (optional)
	if s.IC != nil {
		if err := pdf.CheckVersion(rm.Out, "square annotation IC entry", pdf.V1_4); err != nil {
			return nil, err
		}
		icArray := make(pdf.Array, len(s.IC))
		for i, color := range s.IC {
			icArray[i] = pdf.Number(color)
		}
		dict["IC"] = icArray
	}

	// BE (optional)
	if s.BE != 0 {
		if err := pdf.CheckVersion(rm.Out, "square annotation BE entry", pdf.V1_5); err != nil {
			return nil, err
		}
		dict["BE"] = s.BE
	}

	// RD (optional)
	if len(s.RD) == 4 {
		if err := pdf.CheckVersion(rm.Out, "square annotation RD entry", pdf.V1_5); err != nil {
			return nil, err
		}
		rdArray := make(pdf.Array, 4)
		for i, diff := range s.RD {
			rdArray[i] = pdf.Number(diff)
		}
		dict["RD"] = rdArray
	}

	return dict, nil
}
