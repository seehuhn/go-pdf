// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
			BBox:       pdf.Rectangle{LLx: 0, LLy: 0, URx: 10, URy: 10},
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
