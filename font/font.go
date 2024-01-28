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
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
)

// Glyph represents a single glyph.
type Glyph struct {
	GID glyph.ID

	// Advance is the advance with for the current glyph the client
	// wishes to achieve.  It is measured in PDF text space units,
	// and is already scaled by the font size.
	Advance float64

	// Rise is by how much the glyph should be lifted above the baseline.  The
	// rise is measured in PDF text space units, and is already scaled by the
	// font size.
	Rise float64

	Text []rune
}

// Font represents a font which can be embedded in a PDF file.
type Font interface {
	Embed(w pdf.Putter, resName pdf.Name) (Layouter, error)
}

// A Layouter is a font embedded in a PDF file which can typeset string data.
type Layouter interface {
	Embedded

	Layout(s string) glyph.Seq
	GetGeometry() *Geometry
	FontMatrix() []float64 // TODO(voss): remove

	// CodeAndWidth appends the code for a given glyph/text to s and returns
	// the width of the glyph in PDF text space units (still to be multiplied
	// by the font size). The final return value is true if PDF word spacing
	// adjustment applies to the glyph.
	CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool)

	Close() error
}

// Embedded represents a font which is already embedded in a PDF file.
type Embedded interface {
	Resource
	WritingMode() int // 0 = horizontal, 1 = vertical
	ForeachWidth(s pdf.String, yield func(width float64, is_space bool))
}

// Resource is a PDF resource.
type Resource interface {
	DefaultName() pdf.Name // return "" to choose names automatically
	PDFObject() pdf.Object // value to use in the resource dictionary
}

// Res can be embedded in a struct to implement the [Resource] interface.
type Res struct {
	DefName pdf.Name
	Ref     pdf.Object
}

// DefaultName implements the [Resource] interface.
func (r Res) DefaultName() pdf.Name {
	return r.DefName
}

// PDFObject implements the [Resource] interface.
func (r Res) PDFObject() pdf.Object {
	return r.Ref
}
