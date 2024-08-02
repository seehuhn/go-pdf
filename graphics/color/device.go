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

// == DeviceGray =============================================================

// spaceDeviceGray represents the DeviceGray color space.
type spaceDeviceGray struct{}

// Embed implements the [Space] interface.
func (s spaceDeviceGray) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused
	if err := pdf.CheckVersion(rm.Out, "DeviceGray color space", pdf.V1_1); err != nil {
		return nil, zero, err
	}
	return FamilyDeviceGray, zero, nil
}

// ColorSpaceFamily implements the [SpaceEmbedded] interface.
func (s spaceDeviceGray) ColorSpaceFamily() pdf.Name {
	return FamilyDeviceGray
}

// defaultValues implements the [SpaceEmbedded] interface.
func (s spaceDeviceGray) defaultValues() []float64 {
	return []float64{0}
}

// Default returns the black in the DeviceGray color space.
// This implements the [Space] interface.
func (s spaceDeviceGray) Default() Color {
	return colorDeviceGray(0)
}

type colorDeviceGray float64

// DeviceGray returns a color in the DeviceGray color space.
// The parameter gray must be in the range from 0 (black) to 1 (white).
func DeviceGray(gray float64) Color {
	return colorDeviceGray(gray)
}

// ColorSpace implements the [Color] interface.
func (c colorDeviceGray) ColorSpace() Space {
	return spaceDeviceGray{}
}

// values implements the [Color] interface.
func (c colorDeviceGray) values() []float64 {
	return []float64{float64(c)}
}

// == DeviceRGB ==============================================================

// spaceDeviceRGB represents the DeviceRGB color space.
type spaceDeviceRGB struct{}

// Embed implements the [Space] interface.
func (s spaceDeviceRGB) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused
	if err := pdf.CheckVersion(rm.Out, "DeviceRGB color space", pdf.V1_1); err != nil {
		return nil, zero, err
	}
	return FamilyDeviceRGB, zero, nil
}

// ColorSpaceFamily implements the [SpaceEmbedded] interface.
func (s spaceDeviceRGB) ColorSpaceFamily() pdf.Name {
	return FamilyDeviceRGB
}

// defaultValues implements the [SpaceEmbedded] interface.
func (s spaceDeviceRGB) defaultValues() []float64 {
	return []float64{0, 0, 0}
}

// Default returns the black in the DeviceRGB color space.
// This implements the [Space] interface.
func (s spaceDeviceRGB) Default() Color {
	return colorDeviceRGB{0, 0, 0}
}

type colorDeviceRGB [3]float64

// DeviceRGB returns a color in the DeviceRGB color space.
// The parameters r, g, and b must be in the range from 0 to 1.
func DeviceRGB(r, g, b float64) Color {
	return colorDeviceRGB{r, g, b}
}

// ColorSpace implements the [Color] interface.
func (c colorDeviceRGB) ColorSpace() Space {
	return spaceDeviceRGB{}
}

// values implements the [Color] interface.
func (c colorDeviceRGB) values() []float64 {
	return c[:]
}

// == DeviceCMYK =============================================================

// spaceDeviceCMYK represents the DeviceCMYK color space.
type spaceDeviceCMYK struct{}

// Embed implement the [pdf.Embedder] interface.
func (s spaceDeviceCMYK) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused
	if err := pdf.CheckVersion(rm.Out, "DeviceCMYK color space", pdf.V1_1); err != nil {
		return nil, zero, err
	}

	return FamilyDeviceCMYK, zero, nil
}

// ColorSpaceFamily implements the [Space] interface.
func (s spaceDeviceCMYK) ColorSpaceFamily() pdf.Name {
	return "DeviceCMYK"
}

// defaultValues implements the [Space] interface.
func (s spaceDeviceCMYK) defaultValues() []float64 {
	return []float64{0, 0, 0, 1}
}

// Default returns the black in the DeviceCMYK color space.
// This implements the [Space] interface.
func (s spaceDeviceCMYK) Default() Color {
	return colorDeviceCMYK{0, 0, 0, 1}
}

type colorDeviceCMYK [4]float64

// DeviceCMYK returns a color in the DeviceCMYK color space.
// The parameters c, m, y, and k must be in the range from 0 to 1
// and control the amount of cyan, magenta, yellow, and black in the color.
func DeviceCMYK(c, m, y, k float64) Color {
	return colorDeviceCMYK{c, m, y, k}
}

// ColorSpace implements the [Color] interface.
func (c colorDeviceCMYK) ColorSpace() Space {
	return spaceDeviceCMYK{}
}

// values implements the [Color] interface.
func (c colorDeviceCMYK) values() []float64 {
	return c[:]
}
