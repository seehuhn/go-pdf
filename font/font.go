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

// Append modifies s by appending the glyphs from other.
func (s *GlyphSeq) Append(other *GlyphSeq) {
	if len(s.Seq) == 0 {
		s.Skip += other.Skip
	} else {
		s.Seq[len(s.Seq)-1].Advance += other.Skip
	}
	s.Seq = append(s.Seq, other.Seq...)
}

// TotalWidth returns the total advance width of the glyph sequence.
func (s *GlyphSeq) TotalWidth() float64 {
	w := s.Skip
	for _, g := range s.Seq {
		w += g.Advance
	}
	return w
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

// Embedder represents a font which can be embedded in a PDF file.
type Embedder interface {
	Embed(w pdf.Putter, opt *Options) (Layouter, error)
}

// A Layouter is a font embedded in a PDF file which can typeset text.
type Layouter interface {
	Embedded

	// GetGeometry returns font metrics required for typesetting.
	GetGeometry() *Geometry

	// Layout turns a string into a sequence of glyphs.
	Layout(ptSize float64, s string) *GlyphSeq

	// CodeAndWidth converts a glyph ID (corresponding to the given text) into
	// a PDF character code The character code is appended to s. The function
	// returns the new string s, the width of the glyph in PDF text space units
	// (still to be multiplied by the font size), and a value indicating
	// whether PDF word spacing adjustment applies to this glyph.
	//
	// As a side effect, this function allocates codes for the given
	// glyph/text combination in the font's encoding.
	CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool)

	// Close writes the used subset of the font to the PDF file. After close
	// has been called, only previously used glyph/text combinations can be
	// used.
	//
	// If this function is not called by the user, the font will be
	// automatically closed when the PDF file is closed.
	Close() error
}

// EncodeText encodes a string as a PDF string using the given layouter.
// This allocates character codes as needed.
// All layout information (including kerning) is ignored.
func EncodeText(F Layouter, s string) pdf.String {
	gg := F.Layout(10, s)
	var res pdf.String
	for _, g := range gg.Seq {
		res, _, _ = F.CodeAndWidth(res, g.GID, g.Text)
	}
	return res
}

// Embedded represents a font which is already embedded in a PDF file.
type Embedded interface {
	pdf.Resource
	WritingMode() int // 0 = horizontal, 1 = vertical
	ForeachWidth(s pdf.String, yield func(width float64, is_space bool))
}
