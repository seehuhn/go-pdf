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

// WritingMode is the "writing mode" of a PDF font (horizontal or vertical).
type WritingMode int

const (
	// Horizontal means that text is written left-to-right.
	Horizontal WritingMode = iota

	// Vertical means that text is written top-to-bottom.
	Vertical
)

// Font represents a font instance which can be embedded in a PDF file.
type Font interface {
	PostScriptName() string

	WritingMode() WritingMode

	Embed(rm *pdf.ResourceManager) (Embedded, error)
}

// Embedded represents a font which is already embedded in a PDF file.
type Embedded interface {
	pdf.Resource

	ForeachWidth(s pdf.String, yield func(width float64, isSpace bool))

	// CodeAndWidth converts a glyph ID (corresponding to the given text) into
	// a PDF character code The character code is appended to s. The function
	// returns the new string s, the width of the glyph in PDF text space units
	// (still to be multiplied by the font size), and a value indicating
	// whether PDF word spacing adjustment applies to this glyph.
	//
	// As a side effect, this function allocates codes for the given
	// glyph/text combination in the font's encoding.
	CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool)
}

// A Layouter is a font embedded in a PDF file which can typeset text.
type Layouter interface {
	Font

	// GetGeometry returns font metrics required for typesetting.
	GetGeometry() *Geometry

	// Layout appends a string to a glyph sequence.  The string is typeset at
	// the given point size.
	//
	// If seq is non-nil, a new glyph sequence is allocated.  The resulting
	// GlyphSeq is returned.  If seq is not nil, the return value is guaranteed
	// to be equal to seq.
	Layout(seq *GlyphSeq, ptSize float64, s string) *GlyphSeq
}

// Dict is the low-level interface to represent a font in a PDF file.
//
// TODO(voss): remove?  move somewhere else?  make better use of this?
// merge with Font?
type Dict interface {
	Embed(w pdf.Putter, fontDictRef pdf.Reference) error
}

// Glyph represents a single glyph.
type Glyph struct {
	GID glyph.ID

	// Advance is the advance width for the current glyph the client
	// wishes to achieve.  It is measured in PDF text space units,
	// and is already scaled by the font size.
	Advance float64

	// Rise is by how much the glyph should be lifted above the baseline.  The
	// rise is measured in PDF text space units, and is already scaled by the
	// font size.
	Rise float64

	Text []rune
}

// GlyphSeq represents a sequence of glyphs.
type GlyphSeq struct {
	Skip float64
	Seq  []Glyph
}

// Reset resets the glyph sequence to an empty sequence.
func (s *GlyphSeq) Reset() {
	if s == nil {
		return
	}
	s.Skip = 0
	s.Seq = s.Seq[:0]
}

// TotalWidth returns the total advance width of the glyph sequence.
func (s *GlyphSeq) TotalWidth() float64 {
	w := s.Skip
	for _, g := range s.Seq {
		w += g.Advance
	}
	return w
}

// Text returns the text represented by the glyph sequence.
func (s *GlyphSeq) Text() string {
	n := 0
	for _, g := range s.Seq {
		n += len(g.Text)
	}
	res := make([]rune, 0, n)

	for _, g := range s.Seq {
		res = append(res, g.Text...)
	}

	return string(res)
}

// Append modifies s by appending the glyphs from other.
func (s *GlyphSeq) Append(other *GlyphSeq) {
	if len(s.Seq) == 0 {
		s.Skip += other.Skip
	} else {
		s.Seq[len(s.Seq)-1].Advance += other.Skip
	}
	s.Seq = append(s.Seq, other.Seq...)
}

// Align places the glyphs in a space of the given width.
// q=0 means left alignment, q=1 means right alignment
// and q=0.5 means centering.
func (s *GlyphSeq) Align(width float64, q float64) {
	if len(s.Seq) == 0 {
		return
	}
	extra := width - s.TotalWidth()
	s.Skip += extra * q
	s.Seq[len(s.Seq)-1].Advance += extra * (1 - q)
}

// PadTo modifies s by adding space to the right so that the total width is
// at least width.
func (s *GlyphSeq) PadTo(width float64) {
	if len(s.Seq) == 0 {
		s.Skip = width
		return
	}
	extra := width - s.TotalWidth()
	if extra > 0 {
		s.Seq[len(s.Seq)-1].Advance += extra
	}
}
