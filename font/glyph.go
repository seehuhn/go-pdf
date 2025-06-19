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

package font

import (
	"strings"

	"seehuhn.de/go/sfnt/glyph"
)

// Glyph represents a single glyph.
type Glyph struct {
	// GID identifies the glyph within the font.
	GID glyph.ID

	// Advance is the advance width for the current glyph the client
	// wishes to achieve.  It is measured in PDF text space units,
	// and is already scaled by the font size.
	Advance float64

	// Rise is by how much the glyph should be lifted above the baseline.  The
	// rise is measured in PDF text space units, and is already scaled by the
	// font size.
	Rise float64

	// Text is the text content of the glyph.
	Text string
}

// GlyphSeq represents a sequence of glyphs.
//
// TODO(voss): include the font and size in the struct?
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
	var res strings.Builder
	for _, g := range s.Seq {
		res.WriteString(g.Text)
	}
	return res.String()
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

// Typesetter uses a font together with PDF-specific text layout parameters to
// convert strings into glyph sequences.
type Typesetter struct {
	font              Layouter
	fontSize          float64
	characterSpacing  float64
	wordSpacing       float64
	horizontalScaling float64
	textRise          float64
}

// NewTypesetter creates a new typesetter for the given font and font size.
func NewTypesetter(font Layouter, fontSize float64) *Typesetter {
	return &Typesetter{
		font:              font,
		fontSize:          fontSize,
		horizontalScaling: 1,
	}
}

// SetCharacterSpacing sets the character spacing for the typesetter.
//
// This corresponds to the "Tc" parameter in PDF.  A value of 0 indicates
// normal spacing.  Positive values increase the spacing between characters,
// while negative values decrease it.  The value is used as is and does not
// scale with the font size.
func (t *Typesetter) SetCharacterSpacing(spacing float64) {
	t.characterSpacing = spacing
}

// SetWordSpacing sets the word spacing for the typesetter.
//
// This corresponds to the "Tw" parameter in PDF.  A value of 0 indicates
// normal spacing.  Positive values increase the spacing between words, while
// negative values decrease it.  The value is used as is and does not scale
// with the font size.
func (t *Typesetter) SetWordSpacing(spacing float64) {
	t.wordSpacing = spacing
}

// SetHorizontalScaling sets the horizontal scaling for the typesetter.
//
// This corresponds to the "Tz" parameter in PDF.  A value of 1 indicates
// normal scaling.  Values between 0 and 1 compress the text horizontally,
// while values greater than 1 stretch it.
func (t *Typesetter) SetHorizontalScaling(scaling float64) {
	t.horizontalScaling = scaling
}

// SetTextRise sets the text rise for the typesetter.
//
// This corresponds to the "Ts" parameter in PDF.  The rise is measured in PDF
// text space units, and is already scaled by the font size.
func (t *Typesetter) SetTextRise(rise float64) {
	t.textRise = rise
}

// Layout converts a string into a glyph sequence.
func (t *Typesetter) Layout(seq *GlyphSeq, text string) *GlyphSeq {
	if seq == nil {
		seq = &GlyphSeq{}
	}
	base := len(seq.Seq)

	if t.characterSpacing == 0 {
		t.font.Layout(seq, t.fontSize, text)
	} else { // disable ligatures
		for _, r := range text {
			t.font.Layout(seq, t.fontSize, string(r))
		}
	}

	// Apply PDF layout parameters
	for i := base; i < len(seq.Seq); i++ {
		advance := seq.Seq[i].Advance
		advance += t.characterSpacing
		if seq.Seq[i].Text == " " {
			advance += t.wordSpacing
		}
		seq.Seq[i].Advance = advance * t.horizontalScaling
		seq.Seq[i].Rise = t.textRise
	}

	return seq
}
