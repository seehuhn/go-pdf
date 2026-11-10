// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pdfenc

// A CharacterSet is a collection of glyph names.
type CharacterSet struct {
	Has map[string]bool
}

// StandardLatin is the Adobe Standard Latin character set.
// See appendix D.2 ("Latin character set and encodings") of ISO 32000-2:2020.
var StandardLatin = CharacterSet{
	Has: standardLatinHas,
}

// IsNonSymbolic returns true if all glyphs are in the Adobe Standard Latin
// character set.
func IsNonSymbolic(glyphNames []string) bool {
	for _, name := range glyphNames {
		if !StandardLatin.Has[name] {
			return false
		}
	}
	return true
}
