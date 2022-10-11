// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package font

import (
	"seehuhn.de/go/pdf/sfnt"
	"seehuhn.de/go/pdf/sfnt/cff"
)

// MakeFlags returns the PDF font flags for the font.
// See section 9.8.2 of PDF 32000-1:2008.
func MakeFlags(info *sfnt.Info, symbolic bool) Flags {
	var flags Flags

	if info.IsFixedPitch() {
		flags |= FlagFixedPitch
	}
	if info.IsSerif {
		flags |= FlagSerif
	}

	if symbolic {
		flags |= FlagSymbolic
	} else {
		flags |= FlagNonsymbolic
	}

	if info.IsScript {
		flags |= FlagScript
	}
	if info.IsItalic {
		flags |= FlagItalic
	}

	if cffInfo, ok := info.Outlines.(*cff.Outlines); ok {
		if cffInfo.Private[0].ForceBold {
			flags |= FlagForceBold
		}
	}

	return flags
}

// Flags represents PDF Font Descriptor Flags.
// See section 9.8.2 of PDF 32000-1:2008.
type Flags uint32

// Possible values for PDF Font Descriptor Flags.
const (
	FlagFixedPitch  Flags = 1 << 0  // All glyphs have the same width (as opposed to proportional or variable-pitch fonts, which have different widths).
	FlagSerif       Flags = 1 << 1  // Glyphs have serifs, which are short strokes drawn at an angle on the top and bottom of glyph stems. (Sans serif fonts do not have serifs.)
	FlagSymbolic    Flags = 1 << 2  // Font contains glyphs outside the Adobe standard Latin character set. This flag and the Nonsymbolic flag shall not both be set or both be clear.
	FlagScript      Flags = 1 << 3  // Glyphs resemble cursive handwriting.
	FlagNonsymbolic Flags = 1 << 5  // Font uses the Adobe standard Latin character set or a subset of it.
	FlagItalic      Flags = 1 << 6  // Glyphs have dominant vertical strokes that are slanted.
	FlagAllCap      Flags = 1 << 16 // Font contains no lowercase letters; typically used for display purposes, such as for titles or headlines.
	FlagSmallCap    Flags = 1 << 17 // Font contains both uppercase and lowercase letters.  The uppercase letters are similar to those in the regular version of the same typeface family. The glyphs for the lowercase letters have the same shapes as the corresponding uppercase letters, but they are sized and their proportions adjusted so that they have the same size and stroke weight as lowercase glyphs in the same typeface family.
	FlagForceBold   Flags = 1 << 18 // ...
)
