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
	"bytes"
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/graphics"
)

type Font struct {
	Ascent             funit.Int16
	Descent            funit.Int16
	BaseLineSkip       funit.Int16
	UnderlinePosition  funit.Float64
	UnderlineThickness funit.Float64
	ItalicAngle        float64
	IsFixedPitch       bool
	IsSerif            bool
	IsScript           bool
	IsItalic           bool
	IsAllCap           bool
	IsSmallCap         bool
	ForceBold          bool

	Glyphs     map[string]*Glyph
	FontMatrix [6]float64
	Resources  *pdf.Resources

	glyphNames []string
	cmap       map[rune]glyph.ID
	numOpen    int
}

func New(unitsPerEm uint16) *Font {
	m := [6]float64{
		1 / float64(unitsPerEm), 0,
		0, 1 / float64(unitsPerEm),
		0, 0,
	}
	f := &Font{
		FontMatrix: m,
		Glyphs:     map[string]*Glyph{},
		Resources:  &pdf.Resources{},
		glyphNames: []string{""},
		cmap:       map[rune]glyph.ID{},
	}
	return f
}

func (f *Font) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	if f.numOpen != 0 {
		return nil, fmt.Errorf("font: %d glyphs not closed", f.numOpen)
	}
	res := &embedded{
		w:    w,
		Font: f,
		Resource: pdf.Resource{
			Name: resName,
			Ref:  w.Alloc(),
		},
		SimpleEncoder: cmap.NewSimpleEncoderSequential(),
	}
	w.AutoClose(res)
	return res, nil
}

func (f *Font) GetGeometry() *font.Geometry {
	glyphNames := f.glyphNames

	glyphExtents := make([]funit.Rect16, len(glyphNames))
	widths := make([]funit.Int16, len(glyphNames))
	for i, name := range glyphNames {
		if i == 0 {
			continue
		}
		glyphExtents[i] = f.Glyphs[name].BBox
		widths[i] = f.Glyphs[name].WidthX
	}

	res := &font.Geometry{
		UnitsPerEm:         uint16(math.Round(1 / f.FontMatrix[0])),
		Ascent:             f.Ascent,
		Descent:            f.Descent,
		BaseLineSkip:       f.BaseLineSkip,
		UnderlinePosition:  f.UnderlinePosition,
		UnderlineThickness: f.UnderlineThickness,
		GlyphExtents:       glyphExtents,
		Widths:             widths,
	}
	return res
}

func (f *Font) Layout(s string, ptSize float64) glyph.Seq {
	gg := make(glyph.Seq, 0, len(s))
	for _, r := range s {
		gid, ok := f.cmap[r]
		if !ok {
			gid = glyph.ID(0)
		}
		gg = append(gg, glyph.Info{
			Gid:     gid,
			Text:    []rune{r},
			Advance: f.Glyphs[f.glyphNames[gid]].WidthX,
		})
	}
	return gg
}

// AddGlyph adds a new glyph to the type 3 font.
//
// If shapeOnly is true, a call to the "d1" operator is added at the start of
// the glyph description.  In this case, the glyph description may only specify
// the shape of the glyph, but not its color.  Otherwise, a call to the "d0"
// operator is added at the start of the glyph description.  In this case, the
// glyph description may specify both the shape and the color of the glyph.
func (f *Font) AddGlyph(name string, widthX funit.Int16, bbox funit.Rect16, shapeOnly bool) (*GlyphBuilder, error) {
	if _, exists := f.Glyphs[name]; exists {
		return nil, fmt.Errorf("glyph %q already present", name)
	} else if name == "" {
		return nil, errors.New("empty glyph name")
	}
	f.Glyphs[name] = nil // reserve the name
	if rr := names.ToUnicode(string(name), false); len(rr) == 1 {
		f.cmap[rr[0]] = glyph.ID(len(f.glyphNames))
	}
	f.glyphNames = append(f.glyphNames, name) // this must come after the cmap update
	f.numOpen++

	buf := &bytes.Buffer{}
	page := graphics.NewPage(buf)
	page.ForgetGraphicsState()
	page.Resources = f.Resources

	if shapeOnly {
		fmt.Fprintf(page.Content,
			"%d 0 %d %d %d %d d1\n",
			widthX, bbox.LLx, bbox.LLy, bbox.URx, bbox.URy)
	} else {
		fmt.Fprintf(page.Content, "%d 0 d0\n", widthX)
	}

	g := &GlyphBuilder{
		Page:   page,
		name:   name,
		widthX: widthX,
		bbox:   bbox,
		f:      f,
	}
	return g, nil
}

// GlyphBuilder is used to write a glyph description as described in section
// 9.6.5 of PDF 32000-1:2008.  The .Close() method must be called after
// the description has been written.
type GlyphBuilder struct {
	*graphics.Page
	name   string
	widthX funit.Int16
	bbox   funit.Rect16
	f      *Font
}

// Close most be called after the glyph description has been written.
func (g *GlyphBuilder) Close() error {
	buf := g.Content.(*bytes.Buffer)
	data := &Glyph{
		WidthX: g.widthX,
		BBox:   g.bbox,
		Data:   buf.Bytes(),
	}
	g.f.Glyphs[g.name] = data
	g.f.numOpen--

	return nil
}
