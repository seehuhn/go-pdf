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
	stdcolor "image/color"
	"sync"

	"seehuhn.de/go/icc"
	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 8.6.4

// == DeviceGray =============================================================

// spaceDeviceGray represents the DeviceGray color space.
type spaceDeviceGray struct{}

// Embed adds the color space to a PDF file.
// This implements the [Space] interface.
func (s spaceDeviceGray) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "DeviceGray color space", pdf.V1_1); err != nil {
		return nil, err
	}
	return FamilyDeviceGray, nil
}

// Family returns /DeviceGray.
// This implements the [Space] interface.
func (s spaceDeviceGray) Family() pdf.Name {
	return FamilyDeviceGray
}

// Channels returns 1.
// This implements the [Space] interface.
func (s spaceDeviceGray) Channels() int {
	return 1
}

// Default returns the black in the DeviceGray color space.
// This implements the [Space] interface.
func (s spaceDeviceGray) Default() Color {
	return DeviceGray(0)
}

// Convert converts a color to the DeviceGray color space.
// This implements the [stdcolor.Model] interface.
func (s spaceDeviceGray) Convert(c stdcolor.Color) stdcolor.Color {
	// fast path: already DeviceGray
	if g, ok := c.(DeviceGray); ok {
		return g
	}

	// use luminance formula: 0.299R + 0.587G + 0.114B
	r32, g32, b32, _ := c.RGBA()
	r := float64(r32) / 65535.0
	g := float64(g32) / 65535.0
	b := float64(b32) / 65535.0
	gray := 0.299*r + 0.587*g + 0.114*b
	return DeviceGray(clamp01(gray))
}

// DeviceGray is a color in the DeviceGray color space.
// The value must be in the range from 0 (black) to 1 (white).
type DeviceGray float64

// ColorSpace implements the [Color] interface.
func (c DeviceGray) ColorSpace() Space {
	return spaceDeviceGray{}
}

// ToXYZ returns the colour as CIE XYZ tristimulus values
// adapted to the D50 illuminant.
func (c DeviceGray) ToXYZ() (X, Y, Z float64) {
	v := float64(c)
	return srgbToXYZ(v, v, v)
}

// RGBA implements the color.Color interface.
func (c DeviceGray) RGBA() (r, g, b, a uint32) {
	v := toUint32(float64(c))
	return v, v, v, 0xffff
}

// == DeviceRGB ==============================================================

// PDF 2.0 sections: 8.6.4

// spaceDeviceRGB represents the DeviceRGB color space.
type spaceDeviceRGB struct{}

// Embed adds the color space to a PDF file.
// This implements the [Space] interface.
func (s spaceDeviceRGB) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "DeviceRGB color space", pdf.V1_1); err != nil {
		return nil, err
	}
	return FamilyDeviceRGB, nil
}

// Family returns /DeviceRGB.
// This implements the [Space] interface.
func (s spaceDeviceRGB) Family() pdf.Name {
	return FamilyDeviceRGB
}

// Channels returns 3.
// This implements the [Space] interface.
func (s spaceDeviceRGB) Channels() int {
	return 3
}

// Default returns the black in the DeviceRGB color space.
// This implements the [Space] interface.
func (s spaceDeviceRGB) Default() Color {
	return DeviceRGB{0, 0, 0}
}

// Convert converts a color to the DeviceRGB color space.
// This implements the [stdcolor.Model] interface.
func (s spaceDeviceRGB) Convert(c stdcolor.Color) stdcolor.Color {
	// fast path: already DeviceRGB
	if rgb, ok := c.(DeviceRGB); ok {
		return rgb
	}

	r32, g32, b32, _ := c.RGBA()
	return DeviceRGB{
		float64(r32) / 65535.0,
		float64(g32) / 65535.0,
		float64(b32) / 65535.0,
	}
}

// DeviceRGB is a color in the DeviceRGB color space.
// The values are r, g, and b, and must be in the range from 0 (dark) to 1 (light).
type DeviceRGB [3]float64

// ColorSpace implements the [Color] interface.
func (c DeviceRGB) ColorSpace() Space {
	return spaceDeviceRGB{}
}

// ToXYZ returns the colour as CIE XYZ tristimulus values
// adapted to the D50 illuminant.
func (c DeviceRGB) ToXYZ() (X, Y, Z float64) {
	return srgbToXYZ(c[0], c[1], c[2])
}

// RGBA implements the color.Color interface.
func (c DeviceRGB) RGBA() (r, g, b, a uint32) {
	return toUint32(c[0]), toUint32(c[1]), toUint32(c[2]), 0xffff
}

// == DeviceCMYK =============================================================

// PDF 2.0 sections: 8.6.4

// spaceDeviceCMYK represents the DeviceCMYK color space.
type spaceDeviceCMYK struct{}

// Embed adds the color space to a PDF file.
// This implements the [Space] interface.
func (s spaceDeviceCMYK) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "DeviceCMYK color space", pdf.V1_1); err != nil {
		return nil, err
	}

	return FamilyDeviceCMYK, nil
}

// Family returns /DeviceCMYK.
// This implements the [Space] interface.
func (s spaceDeviceCMYK) Family() pdf.Name {
	return FamilyDeviceCMYK
}

// Channels returns 4.
// This implements the [Space] interface.
func (s spaceDeviceCMYK) Channels() int {
	return 4
}

// Default returns the black in the DeviceCMYK color space.
// This implements the [Space] interface.
func (s spaceDeviceCMYK) Default() Color {
	return DeviceCMYK{0, 0, 0, 1}
}

// Convert converts a color to the DeviceCMYK color space.
// This implements the [stdcolor.Model] interface.
func (s spaceDeviceCMYK) Convert(c stdcolor.Color) stdcolor.Color {
	// fast path: already DeviceCMYK
	if cmyk, ok := c.(DeviceCMYK); ok {
		return cmyk
	}

	// RGB to CMY, then undercolor removal
	r32, g32, b32, _ := c.RGBA()
	r := float64(r32) / 65535.0
	g := float64(g32) / 65535.0
	b := float64(b32) / 65535.0

	cyan := 1 - r
	magenta := 1 - g
	yellow := 1 - b

	// undercolor removal: extract black component
	k := min(cyan, min(magenta, yellow))
	if k >= 1 {
		return DeviceCMYK{0, 0, 0, 1}
	}

	// adjust CMY values
	cyan = (cyan - k) / (1 - k)
	magenta = (magenta - k) / (1 - k)
	yellow = (yellow - k) / (1 - k)

	return DeviceCMYK{
		clamp01(cyan),
		clamp01(magenta),
		clamp01(yellow),
		clamp01(k),
	}
}

// DeviceCMYK is a color in the DeviceCMYK color space.
// The value are c, m, y, and k, and must be in the range from 0 (light) to 1 (dark).
// They control the amount of cyan, magenta, yellow, and black in the color.
type DeviceCMYK [4]float64

// ColorSpace implements the [Color] interface.
func (c DeviceCMYK) ColorSpace() Space {
	return spaceDeviceCMYK{}
}

var (
	cmykFwdOnce      sync.Once
	cmykFwdTransform *icc.Transform
)

// ToXYZ returns the colour as CIE XYZ tristimulus values
// adapted to the D50 illuminant.
// It uses the CGATS001 CMYK profile when available, otherwise falls back
// to a naive CMYK to sRGB conversion.
func (c DeviceCMYK) ToXYZ() (X, Y, Z float64) {
	cmykFwdOnce.Do(func() {
		p, err := icc.Decode(icc.CGATS001Profile)
		if err != nil {
			return
		}
		cmykFwdTransform, _ = icc.NewTransform(p, icc.DeviceToPCS, icc.Perceptual)
	})

	if cmykFwdTransform != nil {
		return cmykFwdTransform.ToXYZ(c[:])
	}

	// fallback: naive CMYK -> sRGB -> XYZ
	cyan, magenta, yellow, black := c[0], c[1], c[2], c[3]
	rf := (1 - cyan) * (1 - black)
	gf := (1 - magenta) * (1 - black)
	bf := (1 - yellow) * (1 - black)
	return srgbToXYZ(rf, gf, bf)
}

// RGBA implements the color.Color interface.
func (c DeviceCMYK) RGBA() (r, g, b, a uint32) {
	X, Y, Z := c.ToXYZ()
	rf, gf, bf := xyzToSRGB(X, Y, Z)
	return toUint32(rf), toUint32(gf), toUint32(bf), 0xffff
}
