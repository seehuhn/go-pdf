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

// Package color implements PDF color spaces and colors.
//
// In PDF, every color is represented by a color space and a set of color
// values.  Some color spaces don't need parameters and can be used directly:
//   - [DeviceGray]: grayscale colors, e.g. DeviceGray(0.5)
//   - [DeviceRGB]: RGB colors, e.g. DeviceRGB{1, 0, 0}
//   - [DeviceCMYK]: CMYK colors, e.g. DeviceCMYK{1, 0, 0, 0}
//   - [PatternColored]: draws a colored pattern
//   - [PatternUncolored]: draws an uncolored pattern in a given color
//   - [SRGB]: sRGB colors (a special case of an ICC-based color)
//
// Other color spaces depend on parameters and need to be created using
// generator functions:
//   - [CalGray]: make a new CalGray color space
//   - [CalRGB]: make a new CalRGB color space
//   - [Lab]: make a new CIE 1976 L*a*b* color space
//   - [ICCBased]: make a new ICC-based color space
//   - [Indexed]: make a new indexed color space
//   - [Separation]: make a new separation color space
//   - [DeviceN]: make a new DeviceN color space
//
// Each color space is represented by an object of type [Space], which
// has a method New() to create colors in that color space.
package color
