// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package squarefont provides standardized test fonts for unit testing font geometry.
//
// This package creates minimal test fonts across different font technologies
// (TrueType, CFF, Type 1, Type 3) that all produce identical PDF glyph space
// values.  The fonts are designed to test unit conversion consistency and font
// geometry calculations.
//
// The values every font in the collection reproduces are the constants in this
// package; [All] lists the fonts themselves, one per technology and units per
// em, including asymmetric font matrices for edge case testing.
//
// # Glyphs
//
// Each font contains exactly three glyphs:
//
//   - GID 0: ".notdef" (blank glyph, width [NotdefWidth])
//   - GID 1: "space" (blank glyph, width [SpaceWidth])
//   - GID 2: "A" (filled square, width [SquareWidth])
//
// The "A" glyph is a simple filled rectangle that forms a 400×400 square
// when rendered in PDF glyph space coordinates.
package squarefont
