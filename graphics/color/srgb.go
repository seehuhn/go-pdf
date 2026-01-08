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
	_ "embed" // for the sRGB ICC profiles

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 8.6.5.5

// spaceSRGB represents the sRGB color space.
// This is a special case of the ICCBased color space.
type spaceSRGB struct{}

// Family returns /ICCBased.
// This implements the [Space] interface.
func (s spaceSRGB) Family() pdf.Name {
	return "ICCBased"
}

// Channels returns 3.
// This implements the [Space] interface.
func (s spaceSRGB) Channels() int {
	return 3
}

// Embed adds the color space to a PDF file.
// This implements the [Space] interface.
func (s spaceSRGB) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	w := rm.Out()
	if err := pdf.CheckVersion(w, "sRGB color space", pdf.V1_3); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"N": pdf.Integer(3),
	}

	sRef := w.Alloc()
	body, err := w.OpenStream(sRef, dict, pdf.FilterFlate{})
	if err != nil {
		return nil, err
	}
	var iccData []byte
	if pdf.GetVersion(w) >= pdf.V1_7 {
		// ICC version 4.2.0 is supported since PDF 1.7
		iccData = sRGBv4
	} else {
		// ICC version 2.1.0 is supported since PDF 1.3
		iccData = sRGBv2
	}
	_, err = body.Write(iccData)
	if err != nil {
		return nil, err
	}
	err = body.Close()
	if err != nil {
		return nil, err
	}

	return pdf.Array{FamilyICCBased, sRef}, nil
}

// Default returns black in the sRGB color space.
// This implements the [Space] interface.
func (s spaceSRGB) Default() Color {
	return colorSRGB{0, 0, 0}
}

type colorSRGB [3]float64

// SRGB returns a color in the sRGB color space.
// The values r, g, and b should be in the range [0, 1].
//
// This is a special case of an ICCBased color space.
//
// See https://en.wikipedia.org/wiki/SRGB for details.
func SRGB(r, g, b float64) Color {
	return colorSRGB{r, g, b}
}

func (c colorSRGB) ColorSpace() Space {
	return spaceSRGB{}
}

//go:embed icc/sRGB-v2-micro.icc
var sRGBv2 []byte

//go:embed icc/sRGB-v4.icc
var sRGBv4 []byte
