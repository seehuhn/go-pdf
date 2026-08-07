// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package extract

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/subset"
)

// resolveFontName settles what the font is called, reconciling the name read
// from the BaseFont entry with the one the font descriptor gives, and writes
// the agreed name back to the descriptor.  It returns the PostScript name and
// the subset tag.
//
// psName and subsetTag are what BaseFont yielded, empty where it gave nothing.
// BaseFont names the font, so the descriptor only supplies what BaseFont left
// out.  embedded reports whether the file carries a font program.
//
// The name is repaired so that it can be written back out: a file may name a
// font in a way a font program cannot, and a tool which rewrites the file
// embeds a program under the name settled here.  Reading is where this happens,
// since a file is not the caller's to fix; a caller which supplies such a name
// through the API has it refused when the font dictionary is written.
func resolveFontName(desc *font.Descriptor, psName, subsetTag string, embedded bool) (string, string) {
	descTag, descName := subset.Split(desc.FontName)
	psName = subset.CleanName(psName)
	descName = subset.CleanName(descName)
	if subsetTag == "" {
		subsetTag = descTag
	}
	if psName == "" {
		psName = descName
	}
	if psName == "" {
		psName = "Font"
	}
	if !subset.IsValidTag(subsetTag) {
		subsetTag = ""
	}

	// External fonts cannot be subsetted, so they must not carry a subset tag.
	// The tag is dropped after the descriptor has been consulted, since
	// otherwise a "TAG+Name" external font would re-acquire one and fail to
	// embed.
	if !embedded {
		subsetTag = ""
	}

	desc.FontName = subset.Join(subsetTag, psName)
	return psName, subsetTag
}
