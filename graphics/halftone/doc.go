// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
