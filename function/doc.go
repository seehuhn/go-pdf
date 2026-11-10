// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
//   - [Type0]: Sampled functions using tables of sample values with interpolation.
//   - [Type2]: Power interpolation functions defining y = C0 + x^N × (C1 - C0).
//   - [Type3]: Piecewise defined 1-input functions.
//   - [Type4]: Functions using PostScript language subset.
//
// All function types implement the [pdf.Function] interface.
package function
