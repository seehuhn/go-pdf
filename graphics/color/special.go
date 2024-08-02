// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package color

import (
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
)

// == Indexed ================================================================

// SpaceIndexed represents an indexed color space.
type SpaceIndexed struct {
	NumCol int

	base   Space
	lookup pdf.String
}

// Indexed returns a new indexed color space.
//
// The colors must all use the same color space, and the number of colors must
// be in the range from 1 to 256 (both inclusive).
func Indexed(colors []Color) (*SpaceIndexed, error) {
	if len(colors) == 0 || len(colors) > 256 {
		return nil, fmt.Errorf("Indexed: invalid number of colors: %d", len(colors))
	}

	space := colors[0].ColorSpace()
	var min, max []float64
	switch space := space.(type) {
	case spaceDeviceGray, *SpaceCalGray:
		min = []float64{0}
		max = []float64{1}
	case spaceDeviceRGB, *SpaceCalRGB:
		min = []float64{0, 0, 0}
		max = []float64{1, 1, 1}
	case spaceDeviceCMYK:
		min = []float64{0, 0, 0, 0}
		max = []float64{1, 1, 1, 1}
	case *SpaceLab:
		min = []float64{0, space.ranges[0], space.ranges[2]}
		max = []float64{100, space.ranges[1], space.ranges[3]}
	case spacePatternColored, spacePatternUncolored, *SpaceIndexed:
		return nil, fmt.Errorf("Indexed: invalid base color space %s", space.ColorSpaceFamily())
	}

	lookup := make(pdf.String, 0, len(colors)*len(min))
	for _, color := range colors {
		if color.ColorSpace() != space {
			return nil, errors.New("Indexed: inconsistent color spaces")
		}
		values := color.values()
		for i, x := range values {
			b := int(math.Floor((x - min[i]) / (max[i] - min[i]) * 256))
			if b < 0 {
				b = 0
			} else if b > 255 {
				b = 255
			}
			lookup = append(lookup, byte(b))
		}
	}

	return &SpaceIndexed{
		base:   space,
		NumCol: len(colors),
		lookup: lookup,
	}, nil
}

// ColorSpaceFamily implements the [Space] interface.
func (s *SpaceIndexed) ColorSpaceFamily() pdf.Name {
	return "Indexed"
}

// New returns a new indexed color.
func (s *SpaceIndexed) New(idx int) Color {
	if idx < 0 || idx >= s.NumCol {
		return nil
	}
	return colorIndexed{Space: s, Index: idx}
}

// Default returns color 0 in the indexed color space.
// This implements the [Space] interface.
func (s *SpaceIndexed) Default() Color {
	return colorIndexed{Space: s, Index: 0}
}

func (s *SpaceIndexed) defaultValues() []float64 {
	return []float64{0}
}

// Embed implements the [Space] interface.
func (s *SpaceIndexed) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused
	if err := pdf.CheckVersion(rm.Out, "Indexed color space", pdf.V1_1); err != nil {
		return nil, zero, err
	}

	base, _, err := pdf.ResourceManagerEmbed(rm, s.base)
	if err != nil {
		return nil, zero, err
	}

	data := pdf.Array{
		pdf.Name("Indexed"),
		base,
		pdf.Integer(s.NumCol - 1),
		s.lookup,
	}

	return data, zero, nil
}

type colorIndexed struct {
	Space *SpaceIndexed
	Index int
}

func (c colorIndexed) ColorSpace() Space {
	return c.Space
}

func (c colorIndexed) values() []float64 {
	return []float64{float64(c.Index)}
}

// == Separation =============================================================

// TODO(voss): implement this

// == DeviceN ================================================================

// TODO(voss): implement this
