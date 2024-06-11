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

import "seehuhn.de/go/pdf"

var (
	// DeviceGray is the DeviceGray color space.
	DeviceGray = SpaceDeviceGray{}

	// DeviceRGB is the DeviceRGB color space.
	DeviceRGB = SpaceDeviceRGB{}

	// DeviceCMYK is the DeviceCMYK color space.
	DeviceCMYK = SpaceDeviceCMYK{}
)

// == DeviceGray =============================================================

// SpaceDeviceGray represents the DeviceGray color space.
// Use [DeviceGray] to access this color space.
type SpaceDeviceGray struct{}

// DefaultName implements the [Space] interface.
func (s SpaceDeviceGray) DefaultName() pdf.Name {
	return ""
}

// PDFObject implements the [Space] interface.
func (s SpaceDeviceGray) PDFObject() pdf.Object {
	return pdf.Name(FamilyDeviceGray)
}

// ColorSpaceFamily implements the [Space] interface.
func (s SpaceDeviceGray) ColorSpaceFamily() pdf.Name {
	return FamilyDeviceGray
}

// defaultColor implements the [Space] interface.
func (s SpaceDeviceGray) defaultColor() Color {
	return colorDeviceGray(0)
}

// New returns a color in the DeviceGray color space.
func (s SpaceDeviceGray) New(gray float64) Color {
	return colorDeviceGray(gray)
}

type colorDeviceGray float64

// ColorSpace implements the [Color] interface.
func (c colorDeviceGray) ColorSpace() Space {
	return DeviceGray
}

// values implements the [Color] interface.
func (c colorDeviceGray) values() []float64 {
	return []float64{float64(c)}
}

// == DeviceRGB ==============================================================

// SpaceDeviceRGB represents the DeviceRGB color space.
// Use [DeviceRGB] to access this color space.
type SpaceDeviceRGB struct{}

// DefaultName implements the [Space] interface.
func (s SpaceDeviceRGB) DefaultName() pdf.Name {
	return ""
}

// PDFObject implements the [Space] interface.
func (s SpaceDeviceRGB) PDFObject() pdf.Object {
	return pdf.Name(FamilyDeviceRGB)
}

// ColorSpaceFamily implements the [Space] interface.
func (s SpaceDeviceRGB) ColorSpaceFamily() pdf.Name {
	return FamilyDeviceRGB
}

// defaultColor implements the [Space] interface.
func (s SpaceDeviceRGB) defaultColor() Color {
	return colorDeviceRGB{0, 0, 0}
}

// New returns a color in the DeviceRGB color space.
func (s SpaceDeviceRGB) New(r, g, b float64) Color {
	return colorDeviceRGB{r, g, b}
}

type colorDeviceRGB [3]float64

// ColorSpace implements the [Color] interface.
func (c colorDeviceRGB) ColorSpace() Space {
	return DeviceRGB
}

// values implements the [Color] interface.
func (c colorDeviceRGB) values() []float64 {
	return c[:]
}

// == DeviceCMYK =============================================================

// SpaceDeviceCMYK represents the DeviceCMYK color space.
// Use [DeviceCMYK] to access this color space.
type SpaceDeviceCMYK struct{}

// DefaultName implements the [Space] interface.
func (s SpaceDeviceCMYK) DefaultName() pdf.Name {
	return ""
}

// PDFObject implements the [Space] interface.
func (s SpaceDeviceCMYK) PDFObject() pdf.Object {
	return pdf.Name("DeviceCMYK")
}

// ColorSpaceFamily implements the [Space] interface.
func (s SpaceDeviceCMYK) ColorSpaceFamily() pdf.Name {
	return "DeviceCMYK"
}

// defaultColor implements the [Space] interface.
func (s SpaceDeviceCMYK) defaultColor() Color {
	return colorDeviceCMYK{0, 0, 0, 1}
}

// New returns a color in the DeviceCMYK color space.
func (s SpaceDeviceCMYK) New(c, m, y, k float64) Color {
	return colorDeviceCMYK{c, m, y, k}
}

type colorDeviceCMYK [4]float64

// ColorSpace implements the [Color] interface.
func (c colorDeviceCMYK) ColorSpace() Space {
	return DeviceCMYK
}

// values implements the [Color] interface.
func (c colorDeviceCMYK) values() []float64 {
	return c[:]
}
