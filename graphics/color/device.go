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

// Embed implements the [Space] interface.
func (s SpaceDeviceGray) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused
	if err := pdf.CheckVersion(rm.Out, "DeviceGray color space", pdf.V1_1); err != nil {
		return nil, zero, err
	}
	return FamilyDeviceGray, zero, nil
}

// ColorSpaceFamily implements the [SpaceEmbedded] interface.
func (s SpaceDeviceGray) ColorSpaceFamily() pdf.Name {
	return FamilyDeviceGray
}

// defaultValues implements the [SpaceEmbedded] interface.
func (s SpaceDeviceGray) defaultValues() []float64 {
	return []float64{0}
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

// Embed implements the [Space] interface.
func (s SpaceDeviceRGB) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused
	if err := pdf.CheckVersion(rm.Out, "DeviceRGB color space", pdf.V1_1); err != nil {
		return nil, zero, err
	}
	return FamilyDeviceRGB, zero, nil
}

// ColorSpaceFamily implements the [SpaceEmbedded] interface.
func (s SpaceDeviceRGB) ColorSpaceFamily() pdf.Name {
	return FamilyDeviceRGB
}

// defaultValues implements the [SpaceEmbedded] interface.
func (s SpaceDeviceRGB) defaultValues() []float64 {
	return []float64{0, 0, 0}
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

// Embed implement the [pdf.Embedder] interface.
func (s SpaceDeviceCMYK) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused
	if err := pdf.CheckVersion(rm.Out, "DeviceCMYK color space", pdf.V1_1); err != nil {
		return nil, zero, err
	}

	return FamilyDeviceCMYK, zero, nil
}

// ColorSpaceFamily implements the [Space] interface.
func (s SpaceDeviceCMYK) ColorSpaceFamily() pdf.Name {
	return "DeviceCMYK"
}

// defaultValues implements the [Space] interface.
func (s SpaceDeviceCMYK) defaultValues() []float64 {
	return []float64{0, 0, 0, 1}
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
