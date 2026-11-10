// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package font implements the foundations of PDF font handling.  Support for
// specific font types is provided by sub-packages.
//
// # Loading/Creating Fonts
//
// This library supports the following font types:
//   - OpenType fonts (.otf files)
//   - TrueType fonts (.ttf files)
//   - Type 1 fonts (.pfa or .pfb files, optionally with the corresponding .afm files)
//   - Type 3 fonts (glyphs drawn using PDF commands)
//
// Normally, fonts are embedded using the convenience functions in
// [seehuhn.de/go/pdf/font/embed].
// If more control is needed, the following functions can be used for the
// different ways of embedding fonts into PDF files as simple fonts:
//   - [seehuhn.de/go/pdf/font/cff.NewSimple]
//   - [seehuhn.de/go/pdf/font/truetype.NewSimple]
//   - [seehuhn.de/go/pdf/font/opentype.NewSimple]
//   - [seehuhn.de/go/pdf/font/type1.New]
//   - [seehuhn.de/go/pdf/font/type3.Font.New]
//
// The following functions can be used to embed fonts as composite fonts:
//   - [seehuhn.de/go/pdf/font/cff.NewComposite]
//   - [seehuhn.de/go/pdf/font/truetype.NewComposite]
//   - [seehuhn.de/go/pdf/font/opentype.NewComposite]
//
// Fonts can be extracted from existing PDF files using
// the [seehuhn.de/go/pdf/font/dict/ExtractFont] function.
//
// # Fonts included in the library
//
//   - The 14 standard PDF fonts: [seehuhn.de/go/pdf/font/standard]
//   - Extended versions of the 14 standard PDF fonts: [seehuhn.de/go/pdf/font/extended]
//   - The Go Font family: [seehuhn.de/go/pdf/font/gofont]
//
// # Data Types for Representing Fonts
//
//   - An [Instance] represents a font which can be embedded in a PDF file.
//   - A [Layouter] is a font which includes enough information to typeset new text.
package font
