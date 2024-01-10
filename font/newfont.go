// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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
	"seehuhn.de/go/pdf"
)

// NewFont represents a font in a PDF document.
//
// TODO(voss): make sure that vertical writing can later be implemented
// without breaking the API.
type NewFont interface {
	DefaultName() pdf.Name // return "" to choose names automatically
	PDFObject() pdf.Object // value to use in the resource dictionary
	WritingMode() int      // 0 = horizontal, 1 = vertical

	AllWidther
}

// AllWidther is an interface for fonts which can return the width of all
// characters in PDF string.
type AllWidther interface {
	// AllWidths returns a function which iterates over all characters in the
	// given string.  The width values are returned in PDF text space units.
	AllWidths(s pdf.String) func(yield func(w float64, isSpace bool) bool) bool
}
