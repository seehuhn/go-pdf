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

package shading

import (
	"testing"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

func TestShadingEqual(t *testing.T) {
	// Create a simple function for shadings that require one
	fn := &function.Type2{
		XMin: 0, XMax: 1,
		C0: []float64{0, 0, 0},
		C1: []float64{1, 1, 1},
		N:  1,
	}

	shadings := []graphics.Shading{
		&Type1{
			ColorSpace: color.SpaceDeviceRGB,
			F:          fn,
		},
		&Type2{
			ColorSpace: color.SpaceDeviceRGB,
			P0:         vec.Vec2{X: 0, Y: 0},
			P1:         vec.Vec2{X: 100, Y: 0},
			F:          fn,
			TMax:       1,
		},
		&Type3{
			ColorSpace: color.SpaceDeviceRGB,
			Center1:    vec.Vec2{X: 50, Y: 50},
			R1:         0,
			Center2:    vec.Vec2{X: 50, Y: 50},
			R2:         50,
			F:          fn,
			TMax:       1,
		},
		&Type4{
			ColorSpace:        color.SpaceDeviceRGB,
			BitsPerCoordinate: 8,
			BitsPerComponent:  8,
			BitsPerFlag:       2,
			Decode:            []float64{0, 100, 0, 100, 0, 1, 0, 1, 0, 1},
			Vertices: []Type4Vertex{
				{X: 0, Y: 0, Flag: 0, Color: []float64{1, 0, 0}},
				{X: 100, Y: 0, Flag: 1, Color: []float64{0, 1, 0}},
				{X: 50, Y: 100, Flag: 2, Color: []float64{0, 0, 1}},
			},
		},
		&Type5{
			ColorSpace:        color.SpaceDeviceRGB,
			BitsPerCoordinate: 8,
			BitsPerComponent:  8,
			VerticesPerRow:    2,
			Decode:            []float64{0, 100, 0, 100, 0, 1, 0, 1, 0, 1},
			Vertices: []Type5Vertex{
				{X: 0, Y: 0, Color: []float64{1, 0, 0}},
				{X: 100, Y: 0, Color: []float64{0, 1, 0}},
			},
		},
		&Type6{
			ColorSpace:        color.SpaceDeviceRGB,
			BitsPerCoordinate: 8,
			BitsPerComponent:  8,
			BitsPerFlag:       2,
			Decode:            []float64{0, 100, 0, 100, 0, 1, 0, 1, 0, 1},
			Patches: []Type6Patch{
				{
					ControlPoints: [12]vec.Vec2{},
					CornerColors:  [][]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {1, 1, 0}},
					Flag:          0,
				},
			},
		},
		&Type7{
			ColorSpace:        color.SpaceDeviceRGB,
			BitsPerCoordinate: 8,
			BitsPerComponent:  8,
			BitsPerFlag:       2,
			Decode:            []float64{0, 100, 0, 100, 0, 1, 0, 1, 0, 1},
			Patches: []Type7Patch{
				{
					ControlPoints: [16]vec.Vec2{},
					CornerColors:  [][]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {1, 1, 0}},
					Flag:          0,
				},
			},
		},
	}

	for i, a := range shadings {
		for j, b := range shadings {
			got := a.Equal(b)
			want := i == j
			if got != want {
				t.Errorf("shadings[%d].Equal(shadings[%d]) = %v, want %v (types: %T vs %T)",
					i, j, got, want, a, b)
			}
		}
	}
}
