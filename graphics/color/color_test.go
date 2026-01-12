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
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
)

// The following types implement the Color interface.
var (
	_ Color = DeviceGray(0)
	_ Color = DeviceRGB{0, 0, 0}
	_ Color = DeviceCMYK{0, 0, 0, 1}
	_ Color = colorCalGray{}
	_ Color = colorCalRGB{}
	_ Color = colorLab{}
	_ Color = colorICCBased{}
	_ Color = colorSRGB{}
	_ Color = colorColoredPattern{}
	_ Color = colorUncoloredPattern{}
	_ Color = colorIndexed{}
	_ Color = colorSeparation{}
	_ Color = colorDeviceN{}
)

// TestColorsComparable verifies that Colors are comparable, and are not
// implemented as pointers.  This is important because we use the "=="
// operator to check for equality of colors.
func TestColorsComparable(t *testing.T) {
	for _, s := range testColorSpaces {
		c1 := s.Default()
		c2 := s.Default()

		if c1 != c2 {
			t.Errorf("equal colors are not equal: %T", c1)
		}
	}
}

// TestValues verifies that the values() function returns correct values
// for all color types.
func TestValues(t *testing.T) {
	// Create test color spaces needed for some color types
	calGray, _ := CalGray(WhitePointD65, nil, 1)
	calRGB, _ := CalRGB(WhitePointD65, nil, nil, nil)
	lab, _ := Lab(WhitePointD65, nil, nil)
	iccBased, _ := ICCBased(sRGBv2, nil)
	indexed, _ := Indexed([]Color{DeviceRGB{0, 0, 0}, DeviceRGB{1, 1, 1}})
	separation, _ := Separation("spot", SpaceDeviceRGB, testTintTransform())
	deviceN, _ := DeviceN([]pdf.Name{"a", "b"}, SpaceDeviceRGB, testDeviceNTransform(), nil)

	tests := []struct {
		name  string
		color Color
		want  []float64
	}{
		{
			name:  "DeviceGray",
			color: DeviceGray(0.5),
			want:  []float64{0.5},
		},
		{
			name:  "DeviceRGB",
			color: DeviceRGB{0.1, 0.2, 0.3},
			want:  []float64{0.1, 0.2, 0.3},
		},
		{
			name:  "DeviceCMYK",
			color: DeviceCMYK{0.1, 0.2, 0.3, 0.4},
			want:  []float64{0.1, 0.2, 0.3, 0.4},
		},
		{
			name:  "CalGray",
			color: calGray.New(0.7),
			want:  []float64{0.7},
		},
		{
			name:  "CalRGB",
			color: calRGB.New(0.1, 0.2, 0.3),
			want:  []float64{0.1, 0.2, 0.3},
		},
		{
			name:  "Lab",
			color: mustColor(lab.New(50, 10, -20)),
			want:  []float64{50, 10, -20},
		},
		{
			name:  "ICCBased",
			color: mustColor(iccBased.New([]float64{0.1, 0.2, 0.3})),
			want:  []float64{0.1, 0.2, 0.3},
		},
		{
			name:  "SRGB",
			color: SRGB(0.4, 0.5, 0.6),
			want:  []float64{0.4, 0.5, 0.6},
		},
		{
			name:  "ColoredPattern",
			color: colorColoredPattern{Pat: nil},
			want:  nil,
		},
		{
			name:  "UncoloredPattern",
			color: colorUncoloredPattern{Pat: nil, Col: DeviceGray(0.25)},
			want:  []float64{0.25},
		},
		{
			name:  "UncoloredPattern/RGB",
			color: colorUncoloredPattern{Pat: nil, Col: DeviceRGB{0.1, 0.2, 0.3}},
			want:  []float64{0.1, 0.2, 0.3},
		},
		{
			name:  "Indexed",
			color: indexed.New(1),
			want:  []float64{1},
		},
		{
			name:  "Separation",
			color: separation.New(0.75),
			want:  []float64{0.75},
		},
		{
			name:  "DeviceN",
			color: deviceN.New([]float64{0.3, 0.7}),
			want:  []float64{0.3, 0.7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := values(tt.color)
			if !floatSlicesEqualTol(got, tt.want, 1e-9) {
				t.Errorf("values() = %v, want %v", got, tt.want)
			}
		})
	}
}

func mustColor(c Color, err error) Color {
	if err != nil {
		panic(err)
	}
	return c
}

func floatSlicesEqualTol(a, b []float64, tol float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		diff := a[i] - b[i]
		if diff < -tol || diff > tol {
			return false
		}
	}
	return true
}

func testTintTransform() pdf.Function {
	return &function.Type2{
		XMin: 0,
		XMax: 1,
		C0:   []float64{1, 1, 1},
		C1:   []float64{0, 0, 0},
		N:    1,
	}
}

func testDeviceNTransform() pdf.Function {
	return &function.Type4{
		Domain:  []float64{0, 1, 0, 1},
		Range:   []float64{0, 1, 0, 1, 0, 1},
		Program: "pop pop 0.5 0.5 0.5",
	}
}
