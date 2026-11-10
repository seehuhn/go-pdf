// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package dict implements reading and writing of PDF font dictionaries.
//
// The package supports all PDF font dictionary types defined in the PDF
// specification. Each struct provides a one-to-one representation of its
// corresponding dictionary type:
//   - [Type1]
//   - [TrueType]
//   - [Type3]
//   - [CIDFontType0]
//   - [CIDFontType2]
//
// All of these structs implement the [Dict] interface.
//
// Simple fonts select glyphs either by name or via the built-in encoding of a
// font. By contrast, composite fonts select glyphs via a character identifier
// (CID). To make simple fonts more consistent with composite fonts, this
// library introduces artificial CIDs for simple fonts. These values are
// defined to be 0 for unused codes, and the character code plus one for all
// other codes.
package dict

import _ "seehuhn.de/go/pdf/font"
