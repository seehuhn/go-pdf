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

// ComponentRange returns the value range of the gray component.
// This implements the [Space] interface.
func (s spaceDeviceGray) ComponentRange(i int) (lo, hi float64) {
	return 0, 1
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

	r32, g32, b32, _ := c.RGBA()
	r := float64(r32) / 65535.0
	g := float64(g32) / 65535.0
	b := float64(b32) / 65535.0
	return DeviceGray(clip01(rgbToGray(r, g, b)))
}

// ToXYZ converts a gray value to CIE XYZ tristimulus values
// adapted to the Profile Connection Space white point.
// A value outside [0, 1] is adjusted to the nearest valid value.
func (s spaceDeviceGray) ToXYZ(values []float64, ws *icc.Workspace) (X, Y, Z float64) {
	v := clip01(values[0])
	return srgbToXYZ(v, v, v)
}

// FromXYZ converts PCS-adapted CIE XYZ to a DeviceGray component value.
//
// The result is written to dst, which must have space for one component.
//
// ws is unused, but must be non-nil for consistency with the other colour
// spaces; the zero value &icc.Workspace{} is valid.
func (s spaceDeviceGray) FromXYZ(X, Y, Z float64, dst []float64, _ *icc.Workspace) {
	dst[0] = rgbToGray(xyzToSRGB(X, Y, Z))
}

// rgbToGray converts sRGB components to a DeviceGray value.
func rgbToGray(r, g, b float64) float64 {
	return 0.299*r + 0.587*g + 0.114*b
}

// rgbToCMYK converts sRGB components to CMYK with undercolour removal.
// The four components are written to dst.
func rgbToCMYK(r, g, b float64, dst []float64) {
	cyan := 1 - r
	magenta := 1 - g
	yellow := 1 - b
	k := min(cyan, magenta, yellow)
	if k >= 1 {
		dst[0], dst[1], dst[2], dst[3] = 0, 0, 0, 1
		return
	}
	dst[0] = (cyan - k) / (1 - k)
	dst[1] = (magenta - k) / (1 - k)
	dst[2] = (yellow - k) / (1 - k)
	dst[3] = k
}

// DeviceGray is a color in the DeviceGray color space.
// The value must be in the range from 0 (black) to 1 (white).
type DeviceGray float64

// ColorSpace implements the [Color] interface.
func (c DeviceGray) ColorSpace() Space {
	return spaceDeviceGray{}
}

// ToXYZ returns the colour as CIE XYZ tristimulus values
// adapted to the Profile Connection Space white point.
func (c DeviceGray) ToXYZ() (X, Y, Z float64) {
	return spaceDeviceGray{}.ToXYZ([]float64{float64(c)}, &icc.Workspace{})
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

// ComponentRange returns the value range of an RGB component.
// This implements the [Space] interface.
func (s spaceDeviceRGB) ComponentRange(i int) (lo, hi float64) {
	return 0, 1
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

// ToXYZ converts RGB values to CIE XYZ tristimulus values
// adapted to the Profile Connection Space white point.
// Values outside [0, 1] are adjusted to the nearest valid value.
func (s spaceDeviceRGB) ToXYZ(values []float64, ws *icc.Workspace) (X, Y, Z float64) {
	return srgbToXYZ(clip01(values[0]), clip01(values[1]), clip01(values[2]))
}

// FromXYZ converts PCS-adapted CIE XYZ to DeviceRGB component values.
//
// The result is written to dst, which must have space for three components.
//
// ws is unused, but must be non-nil for consistency with the other colour
// spaces; the zero value &icc.Workspace{} is valid.
func (s spaceDeviceRGB) FromXYZ(X, Y, Z float64, dst []float64, _ *icc.Workspace) {
	dst[0], dst[1], dst[2] = xyzToSRGB(X, Y, Z)
}

// DeviceRGB is a color in the DeviceRGB color space.
// The values are r, g, and b, and must be in the range from 0 (dark) to 1 (light).
type DeviceRGB [3]float64

// ColorSpace implements the [Color] interface.
func (c DeviceRGB) ColorSpace() Space {
	return spaceDeviceRGB{}
}

// ToXYZ returns the colour as CIE XYZ tristimulus values
// adapted to the Profile Connection Space white point.
func (c DeviceRGB) ToXYZ() (X, Y, Z float64) {
	return spaceDeviceRGB{}.ToXYZ(c[:], &icc.Workspace{})
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

// ComponentRange returns the value range of a CMYK component.
// This implements the [Space] interface.
func (s spaceDeviceCMYK) ComponentRange(i int) (lo, hi float64) {
	return 0, 1
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

	r32, g32, b32, _ := c.RGBA()
	r := float64(r32) / 65535.0
	g := float64(g32) / 65535.0
	b := float64(b32) / 65535.0

	var cmyk [4]float64
	rgbToCMYK(r, g, b, cmyk[:])
	return DeviceCMYK{
		clip01(cmyk[0]),
		clip01(cmyk[1]),
		clip01(cmyk[2]),
		clip01(cmyk[3]),
	}
}

// ToXYZ converts CMYK values to CIE XYZ tristimulus values
// adapted to the Profile Connection Space white point.
// Values outside [0, 1] are adjusted to the nearest valid value.
func (s spaceDeviceCMYK) ToXYZ(values []float64, ws *icc.Workspace) (X, Y, Z float64) {
	cmyk := ws.Scratch(slotClamp, 4)
	for i := range 4 {
		cmyk[i] = clip01(values[i])
	}
	return deviceCMYKToXYZ(cmyk, ws)
}

var (
	cmykOnce      sync.Once
	cmykTransform *icc.Transform
)

// cmykXform returns the cached bidirectional transform for the built-in CMYK
// profile, or nil if it cannot be decoded.
func cmykXform() *icc.Transform {
	cmykOnce.Do(func() {
		p, err := icc.Decode(icc.CMYKProfile)
		if err != nil {
			return
		}
		cmykTransform, _ = icc.NewTransform(p, icc.Perceptual)
	})
	return cmykTransform
}

// FromXYZ converts PCS-adapted CIE XYZ to DeviceCMYK component values.
//
// The result is written to dst, which must have space for four components.
//
// ws supplies reusable scratch buffers to avoid per-call allocation in a
// hot loop; it must be non-nil (the zero value &icc.Workspace{} is valid)
// and must not be used from more than one goroutine at a time.
func (s spaceDeviceCMYK) FromXYZ(X, Y, Z float64, dst []float64, ws *icc.Workspace) {
	if t := cmykXform(); t != nil && t.CanFromXYZ() {
		t.FromXYZ(X, Y, Z, dst[:4], ws)
		return
	}
	r, g, b := xyzToSRGB(X, Y, Z)
	rgbToCMYK(r, g, b, dst)
}

// DeviceCMYK is a color in the DeviceCMYK color space.
// The value are c, m, y, and k, and must be in the range from 0 (light) to 1 (dark).
// They control the amount of cyan, magenta, yellow, and black in the color.
type DeviceCMYK [4]float64

// ColorSpace implements the [Color] interface.
func (c DeviceCMYK) ColorSpace() Space {
	return spaceDeviceCMYK{}
}

// ToXYZ returns the colour as CIE XYZ tristimulus values
// adapted to the Profile Connection Space white point.
// It uses the built-in CMYK profile when available, otherwise falls back
// to a naive CMYK to sRGB conversion.
func (c DeviceCMYK) ToXYZ() (X, Y, Z float64) {
	return spaceDeviceCMYK{}.ToXYZ(c[:], &icc.Workspace{})
}

func deviceCMYKToXYZ(values []float64, ws *icc.Workspace) (X, Y, Z float64) {
	if t := cmykXform(); t != nil && t.CanToXYZ() {
		return t.ToXYZ(values, ws)
	}

	// fallback: naive CMYK -> sRGB -> XYZ
	cyan, magenta, yellow, black := values[0], values[1], values[2], values[3]
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
