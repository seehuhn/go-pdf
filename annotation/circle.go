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
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// PDF 2.0 sections: 12.5.2 12.5.6.2 12.5.6.8

// Circle represents an annotation that displays an ellipse on the page. When
// opened, the annotation displays a pop-up window containing the text of the
// associated note:
//
//   - The location of the ellipse is given by the Common.Rect field and
//     optionally modified by the Margin field.
//   - The border line color is specified by the Common.Color field.
//     If this is nil, no border is drawn.
//   - The border line style is specified by the BorderStyle field.
//     If this is nil, the Common.Border field is used instead.
//     If both are nil, a solid border with width 1 is used.
//     If the border width is 0, no border is drawn.
type Circle struct {
	Common
	Markup

	// Margin (optional) describes the numerical differences between the
	// Common.Rect entry of the annotation and the boundaries of the ellipse.
	//
	// Slice of four numbers: [left, bottom, right, top]
	//
	// If this is unset, the ellipse coincides with Common.Rect.
	//
	// This can be used in case the BorderEffect causes the graphical
	// representation of the ellipse to extend beyond the boundaries of the
	// annotation rectangle.
	//
	// This corresponds to the /RD entry in the PDF annotation dictionary.
	Margin []float64

	// FillColor (optional) is the colour used to fill the ellipse.
	//
	// Only certain color types are allowed:
	//  - colors in the [color.DeviceGray] color space
	//  - colors in the [color.DeviceRGB] color space
	//  - colors in the [color.DeviceCMYK] color space
	//  - the [Transparent] color
	//
	// If this is nil, the ellipse is not filled.
	//
	// This corresponds to the /IC entry in the PDF annotation dictionary.
	FillColor color.Color

	// BorderStyle (optional) is a border style dictionary specifying the line
	// width and dash pattern that is used in drawing the ellipse.
	// The only supported styles are "S" (solid) and "D" (dashed).
	//
	// If this field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle

	// BorderEffect (optional) is a border effect dictionary used in
	// conjunction with the border style dictionary specified by BorderStyle.
	//
	// This corresponds to the /BE entry in the PDF annotation dictionary.
	BorderEffect *BorderEffect
}

var _ Annotation = (*Circle)(nil)

// AnnotationType returns "Circle".
// This implements the [Annotation] interface.
func (c *Circle) AnnotationType() pdf.Name {
	return "Circle"
}

func decodeCircle(x *pdf.Extractor, dict pdf.Dict) (*Circle, error) {
	r := x.R
	circle := &Circle{}

	// Extract common annotation fields
	if err := decodeCommon(x, &circle.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(x, dict, &circle.Markup); err != nil {
		return nil, err
	}

	// Extract circle-specific fields
	// BS (optional)
	if bs, err := pdf.Optional(pdf.ExtractorGet(x, dict["BS"], ExtractBorderStyle)); err != nil {
		return nil, err
	} else {
		circle.BorderStyle = bs
		circle.Border = nil

		// BE (optional)
		if be, err := pdf.Optional(ExtractBorderEffect(r, dict["BE"])); err != nil {
			return nil, err
		} else {
			circle.BorderEffect = be
		}
	}

	// IC (optional)
	if ic, err := pdf.Optional(extractColor(r, dict["IC"])); err != nil {
		return nil, err
	} else {
		circle.FillColor = ic
	}

	// RD (optional)
	if rd, err := pdf.GetFloatArray(r, dict["RD"]); err == nil && len(rd) == 4 {
		circle.Margin = rd
	}

	return circle, nil
}

func (c *Circle) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	dict := pdf.Dict{
		"Subtype": pdf.Name("Circle"),
	}

	if c.BorderStyle != nil {
		if c.Common.Border != nil {
			return nil, errors.New("conflicting border settings")
		}
		if c.BorderStyle.Style == "D" {
			if len(c.BorderStyle.DashArray) == 0 {
				return nil, errors.New("missing dash array")
			}
		} else if len(c.BorderStyle.DashArray) > 0 {
			return nil, errors.New("unexpected dash array")
		}
	} else if c.BorderEffect != nil {
		return nil, errors.New("border effect without border style")
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
		bs, _, err := pdf.ResourceManagerEmbed(rm, c.BorderStyle)
		if err != nil {
			return nil, err
		}
		dict["BS"] = bs
		delete(dict, "Border")
	}

	// BE (optional)
	if c.BorderEffect != nil {
		if err := pdf.CheckVersion(rm.Out, "circle annotation BE entry", pdf.V1_5); err != nil {
			return nil, err
		}
		be, _, err := pdf.ResourceManagerEmbed(rm, c.BorderEffect)
		if err != nil {
			return nil, err
		}
		dict["BE"] = be
	}

	// IC (optional)
	if c.FillColor != nil {
		if err := pdf.CheckVersion(rm.Out, "circle annotation IC entry", pdf.V1_4); err != nil {
			return nil, err
		}
		if icArray, err := encodeColor(c.FillColor); err != nil {
			return nil, err
		} else if icArray != nil {
			dict["IC"] = icArray
		}
	}

	// RD (optional)
	if c.Margin != nil {
		if err := pdf.CheckVersion(rm.Out, "circle annotation RD entry", pdf.V1_5); err != nil {
			return nil, err
		}
		if len(c.Margin) != 4 {
			return nil, errors.New("invalid length for RD array")
		}
		rd := make(pdf.Array, len(c.Margin))
		for i, xi := range c.Margin {
			if xi < 0 {
				return nil, fmt.Errorf("invalid entry %f in RD array", xi)
			}
			rd[i] = pdf.Number(pdf.Round(xi, 4))
		}

		if c.Margin[0]+c.Margin[2] >= c.Rect.Dx() {
			return nil, errors.New("left and right margins exceed rectangle width")
		}
		if c.Margin[1]+c.Margin[3] >= c.Rect.Dy() {
			return nil, errors.New("top and bottom margins exceed rectangle height")
		}
		dict["RD"] = rd
	}

	return dict, nil
}
