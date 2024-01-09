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
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/type1"
)

// NewFont represents a font in a PDF document.
//
// TODO(voss): make sure that vertical writing can later be implemented
// without breaking the API.
type NewFont interface {
	DefaultName() pdf.Name // return "" to choose names automatically
	PDFObject() pdf.Object // value to use in the resource dictionary
	WritingMode() int      // 0 = horizontal, 1 = vertical
	Decode(pdf.String) (charcode.CharCode, int)

	SplitString(pdf.String) []type1.CID // TODO(voss): remove?
	AllWidther
}

type AllWidther interface {
	AllWidths(s pdf.String) func(yield func(w float64, isSpace bool) bool) bool
	GlyphWidth(type1.CID) float64 // TODO(voss): remove
}
