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

// Package function implements PDF functions, which are parameterized
// mathematical transformations that map m input values to n output values.
//
// PDF functions are static, self-contained numerical transformations used
// throughout PDF for color transformations, halftone spot functions, smooth
// shadings, and other mathematical operations. Functions operate within defined
// domains (valid input ranges) and optionally ranges (valid output ranges).
//
// This package supports all PDF function types:
//
//   - [Type0]: Sampled functions using tables of sample values with interpolation
//   - [Type2]: Power interpolation functions defining y = C0 + x^N × (C1 - C0)
//   - [Type3]: Stitching functions combining multiple 1-input functions across subdomains
//   - [Type4]: PostScript calculator functions using PostScript language subset
//
// All function types implement the [pdf.Function] interface.
package function
