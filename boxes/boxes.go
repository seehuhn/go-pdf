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

// A Rule is a solidly filled rectangular region on the page.
type Rule struct {
	BoxExtent
}

// Draw implements the Box interface.
func (obj *Rule) Draw(page *pages.Page, xPos, yPos float64) {
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

// Text represents a typeset string of characters as a Box object.
type Text struct {
	fontRef pdf.Name
	layout  *font.Layout
}

// NewText returns a new Text object.
func NewText(F *font.Font, ptSize float64, text string) *Text {
	layout := F.Typeset(text, ptSize)
	return &Text{
		fontRef: F.Name,
		layout:  layout,
	}
}

// Extent implements the Box interface
func (obj *Text) Extent() *BoxExtent {
	font := obj.layout.Font
	q := obj.layout.FontSize / float64(font.GlyphUnits)

	width := 0.0
	height := math.Inf(-1)
	depth := math.Inf(-1)
	for _, glyph := range obj.layout.Glyphs {
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
func (obj *Text) Draw(page *pages.Page, xPos, yPos float64) {
	font := obj.layout.Font

	page.Println("q")
	page.Println("BT")
	obj.fontRef.PDF(page)
	fmt.Fprintf(page, " %f Tf\n", obj.layout.FontSize)
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
				s.PDF(page)
				page.Println(" Tj")
				data = nil
				return
			}
		}
		data.PDF(page)
		page.Println(" TJ")
		data = nil
	}

	xOffsAuto := 0
	xOffs := 0
	yOffs := 0
	for _, glyph := range obj.layout.Glyphs {
		if glyph.YOffset != yOffs {
			flush()
			yOffs = glyph.YOffset
			page.Printf("%d Ts\n", yOffs)
		}

		xOffsWanted := xOffs + glyph.XOffset

		if xOffsWanted != xOffsAuto {
			// repositioning needed
			flushRun()
			data = append(data, -pdf.Integer(xOffsWanted-xOffsAuto))
		}
		run = append(run, font.Enc(glyph.Gid)...)

		xOffs += glyph.Advance
		xOffsAuto = xOffsWanted + font.Width[glyph.Gid]
	}
	flush()

	page.Println("ET")
	page.Println("Q")
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
