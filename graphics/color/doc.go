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
// values.  Color spaces which don't need paramters automatically exist
// and the package provides functions to generate colors in these color spaces:
//   - [DeviceGray] makes a DeviceGray color
//   - [DeviceRGB] makes a DeviceRGB color
//   - [DeviceCMYK] makes a DeviceCMYK color
//   - [PatternColored] makes a "color" which draws a colored pattern
//   - [PatternUncolored] makes a "color" which draws an uncolored pattern in a given color
//   - [SRGB] makes an sRGB color (a special case of an ICC-based color)
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
// Each color space is represent by an object of class [Space], which
// has an additional method New() to create new colors in that color space.
package color
