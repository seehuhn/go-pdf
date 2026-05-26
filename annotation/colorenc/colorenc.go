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

// Package colorenc converts between the device-colour arrays used in annotation
// dictionaries and the [color.Color] representation.
//
// Several annotation entries encode a colour as an array of 0, 1, 3, or 4
// numbers, where the number of components selects the colour space (none for
// transparent, DeviceGray, DeviceRGB, or DeviceCMYK respectively).
package colorenc

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// Extract converts a device-colour array into a [color.Color].
//
// The number of array elements selects the colour space: 1 for DeviceGray,
// 3 for DeviceRGB, and 4 for DeviceCMYK. A missing object or an empty array
// yields a nil colour.
func Extract(r pdf.Getter, obj pdf.Object) (color.Color, error) {
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
		return nil, nil
	case 1:
		return color.DeviceGray(colors[0]), nil
	case 3:
		return color.DeviceRGB{colors[0], colors[1], colors[2]}, nil
	case 4:
		return color.DeviceCMYK{colors[0], colors[1], colors[2], colors[3]}, nil
	default:
		return nil, pdf.Errorf("invalid color array length: %d", len(colors))
	}
}

// Encode converts a [color.Color] into a device-colour array.
//
// The colour must use the DeviceGray, DeviceRGB, or DeviceCMYK colour space.
// A nil colour yields a nil array.
func Encode(c color.Color) (pdf.Array, error) {
	if c == nil {
		return nil, nil
	}

	s := c.ColorSpace()
	var x []float64
	if s != nil {
		fam := s.Family()
		switch fam {
		case color.FamilyDeviceGray, color.FamilyDeviceRGB, color.FamilyDeviceCMYK:
			x, _ = color.Values(c)
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

// ExtractRGB converts a 3-element device-colour array into a DeviceRGB
// [color.Color]. A missing object or an empty array yields a nil colour;
// arrays of any other length are rejected.
func ExtractRGB(r pdf.Getter, obj pdf.Object) (color.Color, error) {
	c, _ := pdf.GetArray(r, obj)
	if c == nil {
		return nil, nil
	}

	if len(c) == 0 {
		return nil, nil
	}

	if len(c) != 3 {
		return nil, pdf.Errorf("color array must have 3 components for RGB, got %d", len(c))
	}

	colors := make([]float64, 3)
	for i, colorVal := range c {
		if num, err := pdf.GetNumber(r, colorVal); err == nil {
			colors[i] = float64(num)
		}
	}

	return color.DeviceRGB{colors[0], colors[1], colors[2]}, nil
}

// EncodeRGB converts a DeviceRGB [color.Color] into a 3-element colour array.
// A nil colour yields a nil array; colours in any other colour space are
// rejected.
func EncodeRGB(c color.Color) (pdf.Array, error) {
	if c == nil {
		return nil, nil
	}

	s := c.ColorSpace()
	if s == nil {
		return nil, fmt.Errorf("color must be DeviceRGB")
	}

	fam := s.Family()
	if fam != color.FamilyDeviceRGB {
		return nil, fmt.Errorf("color must be DeviceRGB, got %s", fam)
	}

	x, _ := color.Values(c)
	if len(x) != 3 {
		return nil, fmt.Errorf("unexpected number of color components: %d", len(x))
	}

	colorArray := make(pdf.Array, 3)
	for i, v := range x {
		colorArray[i] = pdf.Number(v)
	}
	return colorArray, nil
}
