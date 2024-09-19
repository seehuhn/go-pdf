// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package pdfenc

// A CharacterSet is a collection of glyph names.
type CharacterSet struct {
	Has map[string]bool
}

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
