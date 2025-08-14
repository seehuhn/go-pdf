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

// Circle represents a circle annotation that displays an ellipse on the page.
// When opened, it displays a popup window containing the text of the associated note.
// The ellipse is inscribed within the annotation rectangle defined by the Rect entry.
type Circle struct {
	Common
	Markup

	// BorderStyle (optional) is a border style dictionary specifying the line
	// width and dash pattern that is used in drawing the ellipse.
	//
	// If the BorderStyle field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle

	// IC (optional; PDF 1.4) is an array of numbers in the range 0.0 to 1.0
	// specifying the interior colour with which to fill the annotation's ellipse.
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
	// underlying circle. The four numbers correspond to the differences
	// in default user space between the left, top, right, and bottom
	// coordinates of Rect and those of the circle, respectively.
	RD []float64
}

var _ Annotation = (*Circle)(nil)

// AnnotationType returns "Circle".
// This implements the [Annotation] interface.
func (c *Circle) AnnotationType() pdf.Name {
	return "Circle"
}

func extractCircle(r pdf.Getter, dict pdf.Dict) (*Circle, error) {
	circle := &Circle{}

	// Extract common annotation fields
	if err := decodeCommon(r, &circle.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(r, dict, &circle.Markup); err != nil {
		return nil, err
	}

	// Extract circle-specific fields
	// BS (optional)
	if bs, err := pdf.Optional(ExtractBorderStyle(r, dict["BS"])); err != nil {
		return nil, err
	} else {
		circle.BorderStyle = bs
	}

	// IC (optional)
	if ic, err := pdf.GetFloatArray(r, dict["IC"]); err == nil && len(ic) > 0 {
		circle.IC = ic
	}

	// BE (optional)
	if be, ok := dict["BE"].(pdf.Reference); ok {
		circle.BE = be
	}

	// RD (optional)
	if rd, err := pdf.GetFloatArray(r, dict["RD"]); err == nil && len(rd) == 4 {
		circle.RD = rd
	}

	return circle, nil
}

func (c *Circle) Encode(rm *pdf.ResourceManager) (pdf.Dict, error) {
	dict := pdf.Dict{
		"Subtype": pdf.Name("Circle"),
	}

	// Add common annotation fields
	if err := c.Common.fillDict(rm, dict, isMarkup(c)); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := c.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Add circle-specific fields
	// BS (optional)
	if c.BorderStyle != nil {
		bs, _, err := c.BorderStyle.Embed(rm)
		if err != nil {
			return nil, err
		}
		dict["BS"] = bs
	}

	// IC (optional)
	if c.IC != nil {
		if err := pdf.CheckVersion(rm.Out, "circle annotation IC entry", pdf.V1_4); err != nil {
			return nil, err
		}
		icArray := make(pdf.Array, len(c.IC))
		for i, color := range c.IC {
			icArray[i] = pdf.Number(color)
		}
		dict["IC"] = icArray
	}

	// BE (optional)
	if c.BE != 0 {
		if err := pdf.CheckVersion(rm.Out, "circle annotation BE entry", pdf.V1_5); err != nil {
			return nil, err
		}
		dict["BE"] = c.BE
	}

	// RD (optional)
	if len(c.RD) == 4 {
		if err := pdf.CheckVersion(rm.Out, "circle annotation RD entry", pdf.V1_5); err != nil {
			return nil, err
		}
		rdArray := make(pdf.Array, 4)
		for i, diff := range c.RD {
			rdArray[i] = pdf.Number(diff)
		}
		dict["RD"] = rdArray
	}

	return dict, nil
}
