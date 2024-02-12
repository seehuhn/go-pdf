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

package graphics

import "seehuhn.de/go/pdf"

// ColorSpace represents a PDF color space.
type ColorSpace interface {
	pdf.Resource
	ColorSpaceFamily() ColorSpaceFamily
	DefaultValues() []float64
}

// ColorSpaceFamily is an enumeration of the color space families defined in PDF-2.0.
type ColorSpaceFamily int

// These are the color space families defined in PDF-2.0.
const (
	ColorSpaceDeviceGray ColorSpaceFamily = iota + 1
	ColorSpaceDeviceRGB
	ColorSpaceDeviceCMYK
	ColorSpaceCalGray
	ColorSpaceCalRGB
	ColorSpaceLab
	ColorSpaceICCBased
	ColorSpaceIndexed
	ColorSpacePattern
	ColorSpaceSeparation
	ColorSpaceDeviceN
)

var colMinVersion = map[ColorSpaceFamily]pdf.Version{
	ColorSpaceDeviceGray: pdf.V1_1,
	ColorSpaceDeviceRGB:  pdf.V1_1,
	ColorSpaceDeviceCMYK: pdf.V1_1,
	ColorSpaceCalGray:    pdf.V1_1,
	ColorSpaceCalRGB:     pdf.V1_1,
	ColorSpaceLab:        pdf.V1_1,
	ColorSpaceICCBased:   pdf.V1_3,
	ColorSpaceIndexed:    pdf.V1_1,
	ColorSpacePattern:    pdf.V1_2,
	ColorSpaceSeparation: pdf.V1_2,
	ColorSpaceDeviceN:    pdf.V1_3,
}
