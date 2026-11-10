// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package opentype implements OpenType fonts embedded into PDF files.
//
// Use of this package should be avoided when creating new documents: If the
// given font contains CFF glyphs, then it is more efficient to embed the font
// via [cff.NewSimple] or [cff.NewComposite] instead.  If the given font
// contains TrueType glyphs, [truetype.NewSimple] or [truetype.NewComposite]
// should be used instead of the function from this package.
package opentype
