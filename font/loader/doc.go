// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package loader provides access to font files by PostScript name.  A
// [FontLoader] serves the fonts bundled with the library, and font files
// which the caller registers using [FontLoader.AddFont] or
// [FontLoader.AddFontMap].
package loader

//go:generate go run ./internal/genbuiltin
