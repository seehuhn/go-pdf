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

package boxes

import (
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/pages"
	"seehuhn.de/go/pdf/sfnt/glyph"
)

// Box represents marks on a page within a rectangular area of known size.
type Box interface {
	Extent() *BoxExtent
	Draw(page *pages.Page, xPos, yPos float64)
}

// BoxExtent gives the dimensions of a Box.
type BoxExtent struct {
	Width, Height, Depth float64
	WhiteSpaceOnly       bool
}

func (obj BoxExtent) String() string {
	extra := ""
	if obj.WhiteSpaceOnly {
		extra = "*"
	}
	return fmt.Sprintf("%gx%g%+g%s", obj.Width, obj.Height, obj.Depth, extra)
}

// Extent implements the Box interface.
// This allows for objects to embed a BoxExtent in order to implement the
// part of the Box interface.
func (obj *BoxExtent) Extent() *BoxExtent {
	return obj
}

// A RuleBox is a solidly filled rectangular region on the page.
type RuleBox struct {
	BoxExtent
}

// Rule returns a new rule box (a box filled solid black).
func Rule(width, height, depth float64) Box {
	return &RuleBox{
		BoxExtent: BoxExtent{
			Width:  width,
			Height: height,
			Depth:  depth,
		},
	}
}

// Draw implements the Box interface.
func (obj *RuleBox) Draw(page *pages.Page, xPos, yPos float64) {
	if obj.Width > 0 && obj.Depth+obj.Height > 0 {
		fmt.Fprintf(page, "%f %f %f %f re f\n",
			xPos, yPos-obj.Depth, obj.Width, obj.Depth+obj.Height)
	}
}

// Kern represents a fixed amount of space.
type Kern float64

// Extent implements the Box interface.
func (obj Kern) Extent() *BoxExtent {
	return &BoxExtent{
		Width:          float64(obj),
		Height:         float64(obj),
		WhiteSpaceOnly: true,
	}
}

// Draw implements the Box interface.
func (obj Kern) Draw(page *pages.Page, xPos, yPos float64) {}

// TextBox represents a typeset string of characters as a Box object.
type TextBox struct {
	Font     *font.Font
	FontSize float64
	Glyphs   []glyph.Info
}

// Text returns a new Text object.
func Text(F *font.Font, ptSize float64, text string) *TextBox {
	return &TextBox{
		Font:     F,
		FontSize: ptSize,
		Glyphs:   F.Typeset(text, ptSize),
	}
}

// Extent implements the Box interface
func (obj *TextBox) Extent() *BoxExtent {
	font := obj.Font
	q := obj.FontSize / float64(font.UnitsPerEm)

	width := 0.0
	height := math.Inf(-1)
	depth := math.Inf(-1)
	for _, glyph := range obj.Glyphs {
		width += glyph.Advance.AsFloat(q)

		thisDepth := font.Descent.AsFloat(q)
		thisHeight := font.Ascent.AsFloat(q)
		if font.GlyphExtents != nil {
			bbox := &font.GlyphExtents[glyph.Gid]
			if bbox.IsZero() {
				continue
			}
			thisDepth = -(bbox.LLy + glyph.YOffset).AsFloat(q)
			thisHeight = (bbox.URy + glyph.YOffset).AsFloat(q)
		}
		if thisDepth > depth {
			depth = thisDepth
		}
		if thisHeight > height {
			height = thisHeight
		}
	}

	// TODO(voss): is the following wise?
	if x := font.Ascent.AsFloat(q); height < x {
		height = x
	}
	if x := -font.Descent.AsFloat(q); depth < x {
		depth = x
	}

	return &BoxExtent{
		Width:  width,
		Height: height,
		Depth:  depth,
	}
}

// Draw implements the Box interface.
func (obj *TextBox) Draw(page *pages.Page, xPos, yPos float64) {
	// TODO(voss): use the code in the graphics package instead.
	font := obj.Font

	page.Println("BT")
	_ = font.InstName.PDF(page)
	fmt.Fprintf(page, " %f Tf\n", obj.FontSize)
	fmt.Fprintf(page, "%f %f Td\n", xPos, yPos)

	var run pdf.String
	var data pdf.Array
	flushRun := func() {
		if len(run) > 0 {
			data = append(data, run)
			run = nil
		}
	}
	flush := func() {
		flushRun()
		if len(data) == 0 {
			return
		}
		if len(data) == 1 {
			if s, ok := data[0].(pdf.String); ok {
				_ = s.PDF(page)
				page.Println(" Tj")
				data = nil
				return
			}
		}
		_ = data.PDF(page)
		page.Println(" TJ")
		data = nil
	}

	xOffsFont := 0
	yOffs := 0
	xOffsPDF := 0
	for _, glyph := range obj.Glyphs {
		gid := glyph.Gid
		if int(gid) >= len(font.Widths) {
			gid = 0
		}

		if int(glyph.YOffset) != yOffs {
			flush()
			page.Printf("%.1f Ts\n", glyph.YOffset.AsFloat(obj.FontSize/float64(font.UnitsPerEm)))
			yOffs = int(glyph.YOffset)
		}

		xOffsWanted := xOffsFont + int(glyph.XOffset)

		delta := xOffsWanted - xOffsPDF
		if delta != 0 {
			flushRun()
			deltaScaled := float64(delta) / float64(font.UnitsPerEm) * 1000
			data = append(data, -pdf.Integer(math.Round(deltaScaled)))
			xOffsPDF += delta // TODO(voss): use this
		}
		run = append(run, font.Enc(gid)...)

		xOffsFont += int(glyph.Advance)
		xOffsPDF = xOffsWanted + int(font.Widths[gid])
	}
	flush()

	page.Println("ET")
}

type raiseBox struct {
	Box
	delta float64
}

func (obj raiseBox) Extent() *BoxExtent {
	extent := obj.Box.Extent()
	extent.Height += obj.delta
	extent.Depth -= obj.delta
	return extent
}

func (obj raiseBox) Draw(page *pages.Page, xPos, yPos float64) {
	obj.Box.Draw(page, xPos, yPos+obj.delta)
}

// Raise raises the box by the given amount.
func Raise(delta float64, box Box) Box {
	return raiseBox{
		Box:   box,
		delta: delta,
	}
}

// Ship appends the box to the page tree as a new page.
func Ship(tree *pages.PageTree, box Box) error {
	ext := box.Extent()
	attr := &pages.Attributes{
		MediaBox: &pdf.Rectangle{
			LLx: 0,
			LLy: 0,
			URx: ext.Width,
			URy: ext.Depth + ext.Height,
		},
	}
	page, err := tree.NewPage(attr)
	if err != nil {
		return err
	}
	box.Draw(page, 0, ext.Depth)
	return page.Close()
}

type walker interface {
	Walk(func(Box))
}

// Walk calls fn for every box in the tree rooted at box.
func Walk(box Box, fn func(Box)) {
	fn(box)
	if w, ok := box.(walker); ok {
		w.Walk(func(child Box) {
			Walk(child, fn)
		})
	}
}

// Parameters contains the parameter values used by the layout engine.
type Parameters struct {
	BaseLineSkip float64
}
