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

package pdf

// TextAlign represents the justification of text, used for the /Q entry of
// free text annotations, redaction annotations, and interactive form fields
// containing variable text.
type TextAlign int

const (
	// TextAlignLeft left-justifies text against the left edge.
	TextAlignLeft TextAlign = 0

	// TextAlignCenter centres text between the left and right edges.
	TextAlignCenter TextAlign = 1

	// TextAlignRight right-justifies text against the right edge.
	TextAlignRight TextAlign = 2
)
