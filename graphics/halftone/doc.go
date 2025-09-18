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

// Package halftone implements PDF halftone screening for approximating
// continuous-tone colors on devices that can only produce discrete colors.
//
// Halftone screening divides device space into a grid of halftone cells, where
// each cell simulates gray shades by selectively painting pixels. This package
// supports all PDF halftone types:
//
//   - [Type1]: Spot function based halftones with frequency and angle
//   - [Type5]: Multi-colorant halftones defining separate screens for multiple colorants
//   - [Type6]: Threshold array halftones with zero screen angle using 8-bit values
//   - [Type10]: Angled threshold array halftones supporting non-zero screen angles through two-square decomposition
//   - [Type16]: High-precision threshold array halftones with 16-bit threshold values
//
// All halftone types implement the [graphics.Halftone] interface.
package halftone
