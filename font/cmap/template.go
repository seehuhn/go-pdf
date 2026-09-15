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

package cmap

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
)

// UTF8H and UTF8V are templates for horizontal and vertical writing: a code
// space with no mappings.  A composite font written with one allocates codes
// for its text from the UTF-8 code space, so that the strings in the content
// stream are the text itself, and embeds the CMap this produces.
//
// The templates must not be modified.
var (
	UTF8H = &File{Name: "UTF-8-H", CodeSpaceRange: charcode.UTF8}
	UTF8V = &File{Name: "UTF-8-V", WMode: font.Vertical, CodeSpaceRange: charcode.UTF8}
)
