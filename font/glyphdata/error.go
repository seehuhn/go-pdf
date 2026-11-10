// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package glyphdata

import "errors"

var (
	// ErrNotFound is returned when requested glyph data cannot be found.
	ErrNotFound = errors.New("glyph data not found")

	// ErrWrongType is returned when a font stream has an unexpected type.
	ErrWrongType = errors.New("unexpected font type")

	// ErrInvalidFont is returned when font data is malformed or invalid.
	ErrInvalidFont = errors.New("invalid font data")
)
