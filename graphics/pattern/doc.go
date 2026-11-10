// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package pattern provides functionality for creating PDF patterns. Three
// pattern types are supported:
//   - Colored tiling patterns (PatternType 1, PaintType 1).
//     These patterns repeat periodically in the plane
//     and color is specified as part of the pattern.
//     Use [NewColoredBuilder] to create this type of pattern.
//   - Uncolored tiling patterns (PatternType 1, PaintType 2)
//     These patterns repeat periodically in the plane.
//     The drawing color must specified when the pattern is used.
//     Use [NewUncoloredBuilder] to create this type of pattern.
//   - Shading patterns (PatternType 2)
//     These patterns are non-repeating,
//     the color is specified by a shading object.
//     Use [Type2] structs for pattern objects of this type.
//
// All pattern types implement the [color.Pattern] interface.
package pattern
