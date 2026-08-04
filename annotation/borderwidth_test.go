// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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
	"slices"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestEffectiveBorderWidth(t *testing.T) {
	cases := []struct {
		name string
		a    Annotation
		want float64
	}{
		{
			// nothing was asked for, so no border is drawn
			"neither",
			&Square{},
			0,
		},
		{
			"border only",
			&Square{Common: Common{Border: &Border{Width: 2}}},
			2,
		},
		{
			"the PDF default border",
			&Square{Common: Common{Border: PDFDefaultBorder}},
			1,
		},
		{
			"style only",
			&Square{BorderStyle: &BorderStyle{Width: 3}},
			3,
		},
		{
			// the border array is ignored whenever a style is present
			"style wins over border",
			&Square{
				Common:      Common{Border: &Border{Width: 2}},
				BorderStyle: &BorderStyle{Width: 3},
			},
			3,
		},
		{
			// a style asking for width 0 is not overridden by the array
			"style suppresses the border",
			&Square{
				Common:      Common{Border: &Border{Width: 2}},
				BorderStyle: &BorderStyle{Width: 0},
			},
			0,
		},
		{
			// a type without a border style dictionary uses the array alone
			"no style dictionary",
			&Text{Common: Common{Border: &Border{Width: 4}}},
			4,
		},
		{
			"no style dictionary and no border",
			&Text{},
			0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveBorderWidth(tc.a); got != tc.want {
				t.Errorf("EffectiveBorderWidth = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEffectiveBorderStyle(t *testing.T) {
	cases := []struct {
		name string
		a    Annotation
		want pdf.Name
	}{
		{"neither", &Square{}, "S"},
		{"plain border array", &Square{Common: Common{Border: PDFDefaultBorder}}, "S"},
		{
			// a border array has no style entry, so a dash pattern is the
			// only way it can ask for anything but a solid border
			"dashed border array",
			&Square{Common: Common{Border: &Border{Width: 1, DashArray: []float64{2, 2}}}},
			"D",
		},
		{"style with no style name", &Square{BorderStyle: &BorderStyle{Width: 1}}, "S"},
		{"beveled style", &Square{BorderStyle: &BorderStyle{Width: 1, Style: "B"}}, "B"},
		{
			// the array is ignored whenever a style is present, so its dashes
			// cannot make a solid style dashed
			"solid style over dashed array",
			&Square{
				Common:      Common{Border: &Border{Width: 1, DashArray: []float64{2, 2}}},
				BorderStyle: &BorderStyle{Width: 1, Style: "S"},
			},
			"S",
		},
		{"no style dictionary", &Text{}, "S"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveBorderStyle(tc.a); got != tc.want {
				t.Errorf("EffectiveBorderStyle = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEffectiveBorderDash(t *testing.T) {
	cases := []struct {
		name string
		a    Annotation
		want []float64
	}{
		{"neither", &Square{}, nil},
		{"solid border array", &Square{Common: Common{Border: PDFDefaultBorder}}, nil},
		{
			"dashed border array",
			&Square{Common: Common{Border: &Border{Width: 1, DashArray: []float64{2, 3}}}},
			[]float64{2, 3},
		},
		{
			"dashed style",
			&Square{BorderStyle: &BorderStyle{Width: 1, Style: "D", DashArray: []float64{4, 1}}},
			[]float64{4, 1},
		},
		{
			// the array is ignored whenever a style is present
			"solid style over dashed array",
			&Square{
				Common:      Common{Border: &Border{Width: 1, DashArray: []float64{2, 3}}},
				BorderStyle: &BorderStyle{Width: 1, Style: "S"},
			},
			nil,
		},
		{
			"no style dictionary",
			&Text{Common: Common{Border: &Border{Width: 1, DashArray: []float64{5}}}},
			[]float64{5},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveBorderDash(tc.a); !slices.Equal(got, tc.want) {
				t.Errorf("EffectiveBorderDash = %v, want %v", got, tc.want)
			}
		})
	}
}
