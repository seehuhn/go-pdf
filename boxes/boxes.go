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
	font   pdf.Name
	layout *font.Layout
}

// NewText returns a new Text object.
func NewText(F *font.Font, ptSize float64, text string) *Text {
	layout := F.Typeset(text, ptSize)
	return &Text{
		font:   F.Name,
		layout: layout,
	}
}

// Extent implements the Box interface
func (obj *Text) Extent() *BoxExtent {
	return &BoxExtent{
		Width:  obj.layout.Width,
		Height: obj.layout.Height,
		Depth:  obj.layout.Depth,
	}
}

// Draw implements the Box interface.
func (obj *Text) Draw(page *pages.Page, xPos, yPos float64) {
	if len(obj.layout.Fragments) == 0 {
		return
	}

	// TODO(voss): use "Tj" if len(Fragments)==1

	page.Println("q")
	page.Println("BT")
	obj.font.PDF(page)
	fmt.Fprintf(page, " %f Tf\n", obj.layout.FontSize)
	fmt.Fprintf(page, "%f %f Td\n", xPos, yPos)
	fmt.Fprint(page, "[") // TODO(voss): use a PDF array?
	for i, frag := range obj.layout.Fragments {
		if i > 0 {
			kern := obj.layout.Kerns[i-1]
			fmt.Fprintf(page, " %d ", kern)
		}
		pdf.String(frag).PDF(page)
	}
	fmt.Fprint(page, "] TJ\n")
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
