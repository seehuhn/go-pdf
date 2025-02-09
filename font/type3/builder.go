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

package type3

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt/glyph"
)

// A Builder is used to create a new type 3 font.
//
// Use [Builder.AddGlyph] to add glyphs to the font, and finally call
// [Builder.Finish] method to create the font object.
type Builder struct {
	Glyphs map[string]*Glyph
	rm     *pdf.ResourceManager

	currentGlyph string
	page         *graphics.Writer
}

// NewBuilder creates a new Builder.
//
// Rm is the resource manager to use.  The resulting font cannot be used with
// any other resource manager.
func NewBuilder(rm *pdf.ResourceManager) *Builder {
	// TODO(voss):
	// - consider the discussion at https://pdf-issues.pdfa.org/32000-2-2020/clause07.html#H7.8.3
	// - check where different PDF versions put the Resources dictionary
	// - make it configurable whether to use per-glyph resource dictionaries?
	page := graphics.NewWriter(nil, rm)

	return &Builder{
		Glyphs: make(map[string]*Glyph),
		rm:     rm,
		page:   page,
	}
}

// glyphList returns a list of all glyph names in the font.  The list starts
// with the empty string (to avoid allocating GID 0), followed by the glyph
// names in alphabetical order.
func (b *Builder) glyphList() []string {
	glyphNames := maps.Keys(b.Glyphs)
	glyphNames = append(glyphNames, "")
	sort.Strings(glyphNames)
	return glyphNames
}

// Finish creates the font object.
func (b *Builder) Finish(prop *Properties) (*Font, error) {
	if err := b.checkIdle(); err != nil {
		return nil, err
	}

	glyphNames := b.glyphList()

	resources := b.page.Resources

	glyphExtents := make([]rect.Rect, len(glyphNames))
	widths := make([]float64, len(glyphNames))
	for i, name := range glyphNames {
		if i == 0 {
			continue
		}
		glyphExtents[i] = glyphBoxToPDF(b.Glyphs[name].BBox, prop.FontMatrix[:])
		widths[i] = float64(b.Glyphs[name].WidthX) * prop.FontMatrix[0]
	}

	geometry := &font.Geometry{
		Ascent:       float64(prop.Ascent) * prop.FontMatrix[3],
		Descent:      float64(prop.Descent) * prop.FontMatrix[3],
		Leading:      float64(prop.BaseLineSkip) * prop.FontMatrix[3],
		GlyphExtents: glyphExtents,
		Widths:       widths,
	}

	cmap := map[rune]glyph.ID{}
	for gid, name := range glyphNames {
		if rr := names.ToUnicode(string(name), false); len(rr) == 1 {
			cmap[rr[0]] = glyph.ID(gid)
		}
	}

	res := &Font{
		RM:         b.rm,
		Resources:  resources,
		glyphNames: glyphNames,
		Glyphs:     b.Glyphs,
		Properties: prop,
		Geometry:   geometry,
		CMap:       cmap,
	}
	return res, nil
}

func glyphBoxToPDF(b funit.Rect16, M []float64) rect.Rect {
	bPDF := rect.Rect{
		LLx: math.Inf(+1),
		LLy: math.Inf(+1),
		URx: math.Inf(-1),
		URy: math.Inf(-1),
	}
	corners := []struct{ x, y funit.Int16 }{
		{b.LLx, b.LLy},
		{b.LLx, b.URy},
		{b.URx, b.LLy},
		{b.URx, b.URy},
	}
	for _, c := range corners {
		xf := float64(c.x)
		yf := float64(c.y)
		x, y := M[0]*xf+M[2]*yf+M[4], M[1]*xf+M[3]*yf+M[5]
		bPDF.LLx = min(bPDF.LLx, x)
		bPDF.LLy = min(bPDF.LLy, y)
		bPDF.URx = max(bPDF.URx, x)
		bPDF.URy = max(bPDF.URy, y)
	}
	return bPDF
}

func (b *Builder) checkIdle() error {
	if b.currentGlyph != "" {
		return fmt.Errorf("glyph %q not yet closed", b.currentGlyph)
	}
	return nil
}

// AddGlyph adds a new glyph.
//
// If shapeOnly is true, the glyph description only describes the shape of the
// glyph, but not its color.  Otherwise, the glyph description specifies both
// shape and color of the glyph.
//
// Use the methods of the embedded [graphics.Writer] to draw the glyph.
//
// The .Close() method of the returned GlyphBuilder must be called after the
// glyph description has been written.  Only one glyph may be open at a time.
func (b *Builder) AddGlyph(name string, widthX float64, bbox funit.Rect16, shapeOnly bool) (*GlyphBuilder, error) {
	if _, exists := b.Glyphs[name]; exists {
		return nil, fmt.Errorf("glyph %q already present", name)
	} else if name == "" {
		return nil, errors.New("empty glyph name")
	}

	if err := b.checkIdle(); err != nil {
		return nil, err
	}
	b.currentGlyph = name

	buf := &bytes.Buffer{}
	b.page.NewStream(buf)

	// TODO(voss): move this to the graphics package, and restrict the list of
	// allowed operators depending on the choice.
	if shapeOnly {
		fmt.Fprintf(b.page.Content,
			"%f 0 %d %d %d %d d1\n",
			widthX, bbox.LLx, bbox.LLy, bbox.URx, bbox.URy)
	} else {
		fmt.Fprintf(b.page.Content, "%f 0 d0\n", widthX)
	}

	g := &GlyphBuilder{
		Writer: b.page,
		widthX: widthX,
		bbox:   bbox,
		b:      b,
	}
	return g, nil
}

// GlyphBuilder is used to write a glyph description as described in section
// 9.6.5 of PDF 32000-1:2008.  The .Close() method must be called after
// the description has been written.
type GlyphBuilder struct {
	*graphics.Writer
	widthX float64
	bbox   funit.Rect16
	b      *Builder
}

// Close most be called after the glyph description has been written.
func (g *GlyphBuilder) Close() error {
	if g.Writer.Err != nil {
		return g.Writer.Err
	}

	b := g.b
	w := b.rm.Out
	ref := w.Alloc()

	body := g.Content.(*bytes.Buffer).Bytes()
	stm, err := w.OpenStream(ref, nil, pdf.FilterCompress{})
	if err != nil {
		return nil
	}
	_, err = stm.Write(body)
	if err != nil {
		return nil
	}
	err = stm.Close()
	if err != nil {
		return nil
	}

	data := &Glyph{
		WidthX: g.widthX,
		BBox:   g.bbox,
		Ref:    ref,
	}
	g.b.Glyphs[b.currentGlyph] = data
	b.currentGlyph = ""

	return nil
}
