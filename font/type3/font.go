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

// Package type3 provides support for embedding type 3 fonts into PDF documents.
package type3

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/sfnt/funit"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/type1/names"
)

// A Builder is used to construct a type 3 font for inclusion in
// PDF file.  Use .AddGlyph() to add the glyphs, and then call .Close()
// to embed the font in the PDF file and to get a font.Font object.
type Builder struct {
	w *pdf.Writer

	unitsPerEm uint16
	glyphWidth []funit.Int16

	cmap      map[rune]glyph.ID
	idxToName map[glyph.ID]pdf.Name
	nameToRef map[pdf.Name]*pdf.Reference
	used      map[glyph.ID]bool
}

// New creates a new Builder for embedding a type 3 font into the PDF file w.
// The canvas for drawing glyphs is the rectangle between (0,0) and
// (width,height).  Often width == height == 1000 is used.
func New(w *pdf.Writer, unitsPerEm uint16) (*Builder, error) {
	t3 := &Builder{
		w: w,

		unitsPerEm: unitsPerEm,
		glyphWidth: make([]funit.Int16, 256),

		cmap:      make(map[rune]glyph.ID),
		idxToName: make(map[glyph.ID]pdf.Name),
		nameToRef: make(map[pdf.Name]*pdf.Reference),
		used:      make(map[glyph.ID]bool),
	}
	return t3, nil
}

// AddGlyph adds a new glyph to the type 3 font.  R is the rune associated with
// the glyph, and width is the horizontal increment in character position.
func (t3 *Builder) AddGlyph(r rune, width funit.Int16) (*Glyph, error) {
	if len(t3.cmap) >= 256 {
		return nil, errors.New("too many glyphs")
	}
	if _, present := t3.cmap[r]; present {
		return nil, errors.New("glyph already present")
	}

	compress := pdf.Name("LZWDecode")
	if t3.w.Version >= pdf.V1_2 {
		compress = "FlateDecode"
	}
	stream, ref, err := t3.w.OpenStream(nil, nil, &pdf.FilterInfo{Name: compress})
	if err != nil {
		return nil, err
	}

	idx := glyph.ID(r % 256)
	for t3.used[idx] {
		idx = (idx + 1) % 256
	}
	name := pdf.Name(names.FromUnicode(r))
	t3.glyphWidth[idx] = width
	t3.cmap[r] = idx
	t3.idxToName[idx] = name
	t3.nameToRef[name] = ref
	t3.used[idx] = true

	glyph := &Glyph{
		w:   bufio.NewWriter(stream),
		stm: stream,
	}
	return glyph, nil
}

// Embed must be used after all glyphs have been added to the font. The
// resulting font.Font object can be used to place the glyphs on pages.
func (t3 *Builder) Embed(instName string) (*font.Font, error) {
	CharProcs := pdf.Dict{}
	for name, ref := range t3.nameToRef {
		CharProcs[name] = ref
	}

	var min glyph.ID = 256
	var max glyph.ID = 0
	for idx := range t3.used {
		if idx < min {
			min = idx
		}
		if idx > max {
			max = idx
		}
	}

	q := 1000 / float64(t3.unitsPerEm)

	var Widths pdf.Array
	for idx := min; idx <= max; idx++ {
		fw := t3.glyphWidth[idx]
		Widths = append(Widths, pdf.Integer(math.Round(float64(fw)*q)))
	}

	var Differences pdf.Array
	var prevIdx glyph.ID = 256
	for idx := min; idx <= max; idx++ {
		name, ok := t3.idxToName[idx]
		if ok {
			if idx != prevIdx+1 {
				Differences = append(Differences, pdf.Integer(idx))
			}
			Differences = append(Differences, name)
			prevIdx = idx
		}
	}
	Encoding := pdf.Dict{
		"Differences": Differences,
	}

	Font := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type3"),
		"FontBBox": &pdf.Rectangle{}, // [0,0,0,0] is always valid
		"FontMatrix": pdf.Array{pdf.Real(1 / float64(t3.unitsPerEm)), pdf.Integer(0),
			pdf.Integer(0), pdf.Real(1 / float64(t3.unitsPerEm)),
			pdf.Integer(0), pdf.Integer(0)},
		"CharProcs": CharProcs,
		"Encoding":  Encoding,
		"FirstChar": pdf.Integer(min),
		"LastChar":  pdf.Integer(max),
		"Widths":    Widths,
	}
	if t3.w.Version == pdf.V1_0 {
		Font["Name"] = pdf.Name(instName)
	}

	// TODO(voss): If the following condition is violated, we need to include a
	// /ToUnicode entry: "the font includes only character names taken from the
	// Adobe standard Latin character set and the set of named characters in
	// the Symbol font".

	FontRef, err := t3.w.Write(Font, nil)
	if err != nil {
		return nil, err
	}

	font := &font.Font{
		InstName: pdf.Name(instName),
		Ref:      FontRef,
		Layout: func(rr []rune) glyph.Seq {
			gg := make(glyph.Seq, len(rr))
			for i, r := range rr {
				gid := t3.cmap[r]
				gg[i].Gid = gid
				gg[i].Text = []rune{r}
				gg[i].Advance = t3.glyphWidth[gid]
			}
			return gg
		},
		Enc: func(gid glyph.ID) pdf.String {
			if !t3.used[gid] {
				return nil
			}
			return []byte{byte(gid)}
		},
		UnitsPerEm: t3.unitsPerEm,
		// TODO(voss): GlyphExtents etc
		Widths: t3.glyphWidth,
	}

	return font, nil
}

// Glyph is used to write a glyph description as described in section
// 9.6.5 of PDF 32000-1:2008.  The .Close() method must be called after
// the description has been written.
type Glyph struct {
	w   *bufio.Writer
	stm io.WriteCloser
}

// Close most be called after the glyph description has been written. If any
// error occured while writing the glyph description, the same error will also
// be returned here.
func (g *Glyph) Close() error {
	err := g.w.Flush()
	if err != nil {
		return err
	}
	g.w = nil
	return g.stm.Close()
}

// Write writes the contents of buf to the content stream.  It returns the
// number of bytes written.  If nn < len(p), it also returns an error
// explaining why the write is short.
func (g *Glyph) Write(buf []byte) (int, error) {
	return g.w.Write(buf)
}

// Print formats the arguments using their default formats and writes the
// resulting string to the content stream.  Spaces are added between operands
// when neither is a string.
func (g *Glyph) Print(a ...interface{}) (int, error) {
	return g.w.WriteString(fmt.Sprint(a...))
}

// Printf formats the arguments according to a format specifier and writes the
// resulting string to the content stream.
func (g *Glyph) Printf(format string, a ...interface{}) (int, error) {
	return g.w.WriteString(fmt.Sprintf(format, a...))
}

// Println formats its arguments using their default formats and writes the
// resulting string to the content stream.  Spaces are always added between
// operands and a newline is appended.
func (g *Glyph) Println(a ...interface{}) (int, error) {
	return g.w.WriteString(fmt.Sprintln(a...))
}
