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

// Circle represents an annotation that displays an ellipse on the page.
// When opened, it displays a popup window containing the text of the associated note.
// The ellipse is inscribed within the annotation rectangle defined by the Rect entry.
type Circle struct {
	Common
	Markup

	// Margin (optional; PDF 1.5) describes the numerical differences between
	// the Rect entry of the annotation and the actual boundaries of the
	// underlying ellipse.
	//
	// Slice of four numbers: [left, bottom, right, top]
	//
	// This is useful in case the BorderEffect causes the graphical
	// representation of the circle to extend beyond the boundaries of the
	// original circle.
	//
	// This corresponds to the /RD entry in the PDF annotation dictionary.
	Margin []float64

	// FillColor (optional; PDF 1.4) is the colour used to fill the ellipse.
	//
	// Only certain color types are allowed:
	//  - colors in the [color.DeviceGray] color space
	//  - colors in the [color.DeviceRGB] color space
	//  - colors in the [color.DeviceCMYK] color space
	//  - the [Transparent] color
	//
	// This corresponds to the /IC entry in the PDF annotation dictionary.
	FillColor color.Color

	// BorderStyle (optional) is a border style dictionary specifying the line
	// width and dash pattern that is used in drawing the ellipse.
	//
	// If the BorderStyle field is set, the Common.Border field is ignored.
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

func decodeCircle(r pdf.Getter, dict pdf.Dict) (*Circle, error) {
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
	if ic, err := pdf.Optional(extractColor(r, dict["IC"])); err != nil {
		return nil, err
	} else {
		circle.FillColor = ic
	}

	// BE (optional)
	if be, err := pdf.Optional(ExtractBorderEffect(r, dict["BE"])); err != nil {
		return nil, err
	} else {
		circle.BorderEffect = be
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
