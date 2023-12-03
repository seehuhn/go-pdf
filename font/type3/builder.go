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
	"regexp"
	"strconv"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/graphics"
)

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
		f.CMap[rr[0]] = glyph.ID(len(f.glyphNames))
	}
	f.glyphNames = append(f.glyphNames, name) // this must come after the cmap update
	f.numOpen++

	buf := &bytes.Buffer{}
	page := graphics.NewWriter(buf, pdf.V1_7) // TODO(voss): what to use as the PDF version here?
	page.Resources = f.Resources

	if shapeOnly {
		fmt.Fprintf(page.Content,
			"%d 0 %d %d %d %d d1\n",
			widthX, bbox.LLx, bbox.LLy, bbox.URx, bbox.URy)
	} else {
		fmt.Fprintf(page.Content, "%d 0 d0\n", widthX)
	}

	g := &GlyphBuilder{
		Writer: page,
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
	*graphics.Writer
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

func setGlyphGeometry(g *Glyph, data []byte) {
	m := type3StartRegexp.FindSubmatch(data)
	if len(m) != 9 {
		return
	}
	if m[1] != nil {
		x, _ := strconv.ParseFloat(string(m[1]), 64)
		g.WidthX = funit.Int16(math.Round(x))
	} else if m[3] != nil {
		var xx [6]funit.Int16
		for i := range xx {
			x, _ := strconv.ParseFloat(string(m[3+i]), 64)
			xx[i] = funit.Int16(math.Round(x))
		}
		g.WidthX = xx[0]
		g.BBox = funit.Rect16{
			LLx: xx[2],
			LLy: xx[3],
			URx: xx[4],
			URy: xx[5],
		}
	}
}

var (
	spc = `[\t\n\f\r ]+`
	num = `([+-]?[0-9.]+)` + spc
	d0  = num + num + "d0"
	d1  = num + num + num + num + num + num + "d1"

	type3StartRegexp = regexp.MustCompile(`^[\t\n\f\r ]*(?:` + d0 + "|" + d1 + ")" + spc)
)
