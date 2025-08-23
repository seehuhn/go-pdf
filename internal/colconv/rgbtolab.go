// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package colconv

import (
	"math"

	"seehuhn.de/go/pdf/graphics/color"
)

const (
	deviceGamma = 2.2
)

// DeviceGrayToL converts a DeviceGray value (0-1 range) to an L* value in the
// LAB color space.  The corresponding A and B values are always 0.
func DeviceGrayToL(gray float64) (L float64) {
	// Apply gamma correction to get linear luminance
	linear := math.Pow(gray, deviceGamma)

	// Gray is equivalent to Y in XYZ (normalized by white point)
	y := linear / color.WhitePointD65[1]

	// Convert Y to L*
	fy := labF(y)
	L = 116*fy - 16

	return L
}

// LToDeviceGray converts a LAB color space L* value to DeviceGray (0-1 range).
func LToDeviceGray(L float64) float64 {
	// Convert L* to Y
	fy := (L + 16) / 116
	y := labFInv(fy) * color.WhitePointD65[1]

	// Clamp linear value
	y = clamp(y, 0, 1)

	// Apply inverse gamma correction
	gray := math.Pow(y, 1/deviceGamma)

	return gray
}

// DeviceRGBToLAB converts RGB values (0-1 range) to LAB color space.
func DeviceRGBToLAB(r, g, b float64) (L, A, B float64) {
	// Apply gamma correction
	r = math.Pow(r, deviceGamma)
	g = math.Pow(g, deviceGamma)
	b = math.Pow(b, deviceGamma)

	// RGB to XYZ using sRGB matrix
	x := 0.4124564*r + 0.3575761*g + 0.1804375*b
	y := 0.2126729*r + 0.7151522*g + 0.0721750*b
	z := 0.0193339*r + 0.1191920*g + 0.9503041*b

	// Normalize by D65 white point
	x = x / color.WhitePointD65[0]
	y = y / color.WhitePointD65[1]
	z = z / color.WhitePointD65[2]

	// XYZ to LAB
	fx := labF(x)
	fy := labF(y)
	fz := labF(z)

	L = 116*fy - 16
	A = 500 * (fx - fy)
	B = 200 * (fy - fz)

	return L, A, B
}

// LABToDeviceRGB converts LAB color space values to RGB (0-1 range).
func LABToDeviceRGB(L, A, B float64) (r, g, b float64) {
	// LAB to XYZ
	fy := (L + 16) / 116
	fx := A/500 + fy
	fz := fy - B/200

	x := labFInv(fx) * color.WhitePointD65[0]
	y := labFInv(fy) * color.WhitePointD65[1]
	z := labFInv(fz) * color.WhitePointD65[2]

	// XYZ to RGB using inverse sRGB matrix
	r = 3.2404542*x + -1.5371385*y + -0.4985314*z
	g = -0.9692660*x + 1.8760108*y + 0.0415560*z
	b = 0.0556434*x + -0.2040259*y + 1.0572252*z

	// clamp
	r = clamp(r, 0, 1)
	g = clamp(g, 0, 1)
	b = clamp(b, 0, 1)

	// Apply inverse gamma correction
	r = math.Pow(r, 1/deviceGamma)
	g = math.Pow(g, 1/deviceGamma)
	b = math.Pow(b, 1/deviceGamma)

	return r, g, b
}

// DeviceCMYKToLAB converts CMYK values (0-1 range) to LAB color space.
func DeviceCMYKToLAB(c, m, y, k float64) (L, A, B float64) {
	// CMYK to RGB using simple device formula
	r := (1 - c) * (1 - k)
	g := (1 - m) * (1 - k)
	b := (1 - y) * (1 - k)

	// Apply gamma correction
	r = math.Pow(r, deviceGamma)
	g = math.Pow(g, deviceGamma)
	b = math.Pow(b, deviceGamma)

	// RGB to XYZ using sRGB matrix
	x := 0.4124564*r + 0.3575761*g + 0.1804375*b
	y2 := 0.2126729*r + 0.7151522*g + 0.0721750*b
	z := 0.0193339*r + 0.1191920*g + 0.9503041*b

	// Normalize by D65 white point
	x = x / color.WhitePointD65[0]
	y2 = y2 / color.WhitePointD65[1]
	z = z / color.WhitePointD65[2]

	// XYZ to LAB
	fx := labF(x)
	fy := labF(y2)
	fz := labF(z)

	L = 116*fy - 16
	A = 500 * (fx - fy)
	B = 200 * (fy - fz)

	return L, A, B
}

// LABToDeviceCMYK converts LAB color space values to CMYK (0-1 range).
func LABToDeviceCMYK(L, A, B float64) (c, m, y, k float64) {
	// LAB to XYZ
	fy := (L + 16) / 116
	fx := A/500 + fy
	fz := fy - B/200

	x := labFInv(fx) * color.WhitePointD65[0]
	y2 := labFInv(fy) * color.WhitePointD65[1]
	z := labFInv(fz) * color.WhitePointD65[2]

	// XYZ to RGB using inverse sRGB matrix
	r := 3.2404542*x + -1.5371385*y2 + -0.4985314*z
	g := -0.9692660*x + 1.8760108*y2 + 0.0415560*z
	b := 0.0556434*x + -0.2040259*y2 + 1.0572252*z

	// Clamp RGB values
	r = clamp(r, 0, 1)
	g = clamp(g, 0, 1)
	b = clamp(b, 0, 1)

	// Apply inverse gamma correction
	r = math.Pow(r, 1/deviceGamma)
	g = math.Pow(g, 1/deviceGamma)
	b = math.Pow(b, 1/deviceGamma)

	// RGB to CMYK
	k = 1 - math.Max(math.Max(r, g), b)

	if k < 1 {
		c = (1 - r - k) / (1 - k)
		m = (1 - g - k) / (1 - k)
		y = (1 - b - k) / (1 - k)
	} else {
		// Pure black
		c = 0
		m = 0
		y = 0
	}

	// Ensure values are in valid range
	c = clamp(c, 0, 1)
	m = clamp(m, 0, 1)
	y = clamp(y, 0, 1)
	k = clamp(k, 0, 1)

	return c, m, y, k
}

func labF(t float64) float64 {
	const delta = 6.0 / 29.0
	if t > delta*delta*delta {
		return math.Pow(t, 1.0/3.0)
	}
	return t/(3*delta*delta) + 4.0/29.0
}

func labFInv(t float64) float64 {
	const delta = 6.0 / 29.0
	if t > delta {
		return t * t * t
	}
	return 3 * delta * delta * (t - 4.0/29.0)
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
