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

package pattern

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/shading"
)

func TestPatternEqual(t *testing.T) {
	patterns := []color.Pattern{
		&Type1{
			TilingType: 1,
			BBox:       &pdf.Rectangle{LLx: 0, LLy: 0, URx: 10, URy: 10},
			XStep:      10,
			YStep:      10,
			Color:      true,
			Content:    nil,
			Res:        &content.Resources{},
		},
		&Type2{
			Shading: &shading.Type1{
				ColorSpace: color.SpaceDeviceGray,
				F: &function.Type2{
					XMin: 0, XMax: 1,
					C0: []float64{0},
					C1: []float64{1},
					N:  1,
				},
			},
		},
	}

	for i, a := range patterns {
		for j, b := range patterns {
			got := a.Equal(b)
			want := i == j
			if got != want {
				t.Errorf("patterns[%d].Equal(patterns[%d]) = %v, want %v (types: %T vs %T)",
					i, j, got, want, a, b)
			}
		}
	}
}
