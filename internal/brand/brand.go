// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package brand holds the Quire design-system palette, used as the default
// colours of generated appearances.  Names and values mirror the CSS custom
// properties in the Quire design system (colors_and_type.css).
package brand

import "seehuhn.de/go/pdf/graphics/color"

var (
	// Ink is the darkest of the warm ink neutrals (--ink, #1a1814), used
	// for text and lines.
	Ink = color.DeviceRGB{0.102, 0.094, 0.078}

	// Ink2 is the second ink neutral (--ink-2, #3a372f).
	Ink2 = color.DeviceRGB{0.227, 0.216, 0.184}

	// Ink3 is the third ink neutral (--ink-3, #6d6858).
	Ink3 = color.DeviceRGB{0.427, 0.408, 0.345}

	// Ink4 is the lightest ink neutral (--ink-4, #9c9684).
	Ink4 = color.DeviceRGB{0.612, 0.588, 0.518}

	// White is pure white, which the design system uses for bevel
	// highlights.
	White = color.DeviceRGB{1, 1, 1}

	// Slate1 is the lightest of the cool neutrals for application chrome
	// (--slate-1, #f5f5f7).
	Slate1 = color.DeviceRGB{0.961, 0.961, 0.969}

	// Slate3 is a darker cool neutral for application chrome (--slate-3,
	// #d9d9dd).
	Slate3 = color.DeviceRGB{0.851, 0.851, 0.867}

	// Amber50 is the lightest of the editorial amber accents (--amber-50,
	// #fdf5e6).
	Amber50 = color.DeviceRGB{0.992, 0.961, 0.902}

	// Amber100 is a light amber accent (--amber-100, #f6e5bf).
	Amber100 = color.DeviceRGB{0.965, 0.898, 0.749}

	// Amber400 is a strong amber accent (--amber-400, #c27518).
	Amber400 = color.DeviceRGB{0.761, 0.459, 0.094}

	// Lapis500 is the academic blue of hyperlinks in prose (--lapis-500,
	// #245a9e).
	Lapis500 = color.DeviceRGB{0.141, 0.353, 0.620}

	// SignalError is the signal colour for errors (--signal-error, #9a2c2c).
	SignalError = color.DeviceRGB{0.604, 0.173, 0.173}
)
