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
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

func extractColor(r pdf.Getter, obj pdf.Object) (color.Color, error) {
	c, _ := pdf.GetArray(r, obj)
	if c == nil {
		return nil, nil
	}

	colors := make([]float64, len(c))
	for i, colorVal := range c {
		if num, err := pdf.GetNumber(r, colorVal); err == nil {
			colors[i] = float64(num)
		}
	}

	switch len(colors) {
	case 0:
		return Transparent, nil
	case 1:
		return color.DeviceGray(colors[0]), nil
	case 3:
		return color.DeviceRGB(colors[0], colors[1], colors[2]), nil
	case 4:
		return color.DeviceCMYK(colors[0], colors[1], colors[2], colors[3]), nil
	default:
		return nil, fmt.Errorf("invalid color array length: %d", len(colors))
	}
}

func encodeColor(c color.Color) (pdf.Array, error) {
	if c == nil {
		return nil, nil
	}

	s := c.ColorSpace()
	var x []float64
	if s != nil {
		fam := s.Family()
		switch fam {
		case color.FamilyDeviceGray, color.FamilyDeviceRGB, color.FamilyDeviceCMYK:
			x, _, _ = color.Operator(c)
		default:
			return nil, fmt.Errorf("unexpected color space %s", fam)
		}
	}

	if len(x) == 0 {
		return nil, nil
	}

	colorArray := make(pdf.Array, len(x))
	for i, v := range x {
		colorArray[i] = pdf.Number(v)
	}
	return colorArray, nil
}
