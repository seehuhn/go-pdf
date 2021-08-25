// seehuhn.de/go/pdf - support for reading and writing PDF files
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
)

// Parameters contains the parameter values used by the layout engine.
type Parameters struct {
	BaseLineSkip float64
}

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

// Extent implements the Box interface.
func (obj BoxExtent) Extent() *BoxExtent {
	return &obj
}

// A RuleBox is a solidly filled rectangular region on the page.
type RuleBox struct {
	BoxExtent
}

// Rule returns a new rule box (a box filled solid black).
func Rule(width, height, depth float64) Box {
	return &RuleBox{
		BoxExtent: BoxExtent{
			Width:          width,
			Height:         height,
			Depth:          depth,
			WhiteSpaceOnly: false,
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
	Layout *font.Layout
}

// Text returns a new Text object.
func Text(F *font.Font, ptSize float64, text string) *TextBox {
	layout, _ := F.Typeset(text, ptSize) // TODO(voss): error handling
	return &TextBox{
		Layout: layout,
	}
}

// Extent implements the Box interface
func (obj *TextBox) Extent() *BoxExtent {
	font := obj.Layout.Font
	q := obj.Layout.FontSize / float64(font.GlyphUnits)

	width := 0.0
	height := math.Inf(-1)
	depth := math.Inf(-1)
	for _, glyph := range obj.Layout.Glyphs {
		width += float64(glyph.Advance) * q

		bbox := &font.GlyphExtent[glyph.Gid]
		if !bbox.IsZero() {
			thisDepth := -float64(bbox.LLy+glyph.YOffset) * q
			if thisDepth > depth {
				depth = thisDepth
			}
			thisHeight := float64(bbox.URy+glyph.YOffset) * q
			if thisHeight > height {
				height = thisHeight
			}
		}
	}

	return &BoxExtent{
		Width:  width,
		Height: height,
		Depth:  depth,
	}
}

// Draw implements the Box interface.
func (obj *TextBox) Draw(page *pages.Page, xPos, yPos float64) {
	obj.Layout.Draw(page, xPos, yPos)
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
	page, err := tree.AddPage(attr)
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
