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

package type3

import (
	"errors"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/sfnt/glyph"
)

// Properties contains global information about a Type 3 font.
type Properties struct {
	FontMatrix matrix.Matrix

	Ascent       funit.Int16
	Descent      funit.Int16
	BaseLineSkip funit.Int16

	ItalicAngle  float64
	IsFixedPitch bool
	IsSerif      bool
	IsScript     bool
	IsAllCap     bool
	IsSmallCap   bool
	ForceBold    bool
}

// Font is a PDF Type 3 font.
//
// Use a [Builder] to create a new font.
type Font struct {
	RM         *pdf.ResourceManager
	Resources  *pdf.Resources
	glyphNames []string
	Glyphs     map[string]*Glyph
	*Properties
	*font.Geometry
	CMap map[rune]glyph.ID
}

// Glyph is a glyph in a type 3 font.
type Glyph struct {
	WidthX float64
	BBox   funit.Rect16 // TODO(voss): use a better type
	Ref    pdf.Reference
}

// PostScriptName returns the empty string (since Type 3 fonts don't have a PostScript name).
// This implements the [font.Font] interface.
func (f *Font) PostScriptName() string {
	return ""
}

// Layout implements the [font.Layouter] interface.
func (f *Font) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	if seq == nil {
		seq = &font.GlyphSeq{}
	}

	q := f.FontMatrix[0] * ptSize

	for _, r := range s {
		gid, ok := f.CMap[r]
		if !ok {
			continue
		}
		seq.Seq = append(seq.Seq, font.Glyph{
			GID:     gid,
			Text:    []rune{r},
			Advance: float64(f.Glyphs[f.glyphNames[gid]].WidthX) * q,
		})
	}
	return seq
}

// Embed implements the [font.Font] interface.
func (f *Font) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	if f.RM != nil && f.RM != rm {
		return nil, nil, errors.New("font from different resource manager")
	}

	glyphNames := f.glyphNames

	w := rm.Out
	ref := w.Alloc()
	res := &embedded{
		Font:          f,
		GlyphNames:    glyphNames,
		w:             w,
		ref:           ref,
		SimpleEncoder: encoding.NewSimpleEncoder(),
	}
	return ref, res, nil
}

type embedded struct {
	*Font
	GlyphNames []string

	w   *pdf.Writer
	ref pdf.Reference

	*encoding.SimpleEncoder
	closed bool
}

// WritingMode implements the [font.Embedded] interface.
func (f *embedded) WritingMode() cmap.WritingMode {
	return 0
}

func (f *embedded) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	c := s[0]
	gid := f.Encoding[c]
	name := f.GlyphNames[gid]
	width := float64(f.Glyphs[name].WidthX) * f.Font.FontMatrix[0]
	return width, 1
}

func (f *embedded) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	name := f.GlyphNames[gid]
	width := float64(f.Glyphs[name].WidthX) * f.Font.FontMatrix[0]
	c := f.GIDToCode(gid, text)
	return append(s, c), width
}

func (f *embedded) Finish(*pdf.ResourceManager) error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		return errors.New("too many distinct glyphs used in Type 3 font")
	}

	glyphs := make(map[pdf.Name]pdf.Reference)
	encoding := make([]string, 256)
	widths := make([]float64, 256)
	for i, gid := range f.Encoding {
		glyphpName := f.GlyphNames[gid]
		if g, ok := f.Glyphs[glyphpName]; ok {
			glyphs[pdf.Name(glyphpName)] = g.Ref
			widths[i] = g.WidthX
			encoding[i] = glyphpName
		}
	}

	isSymbolic := false
	for name := range glyphs {
		if !pdfenc.StandardLatin.Has[string(name)] {
			isSymbolic = true
			break
		}
	}

	fd := &font.Descriptor{
		IsFixedPitch: f.Font.Geometry.IsFixedPitch(),
		IsSerif:      f.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     f.IsScript,
		IsItalic:     f.ItalicAngle != 0,
		IsAllCap:     f.IsAllCap,
		IsSmallCap:   f.IsSmallCap,
		ForceBold:    f.ForceBold,
		ItalicAngle:  f.ItalicAngle,
	}

	res := &dict.Type3{
		Ref:        f.ref,
		FontMatrix: f.FontMatrix,
		CharProcs:  glyphs,
		Encoding:   func(code byte) string { return encoding[code] },
		Descriptor: fd,
		Resources:  f.Resources,
	}
	copy(res.Width[:], widths)
	f.SimpleEncoder.FillText(&res.Text)

	return res.WriteToPDF(f.RM)
}
