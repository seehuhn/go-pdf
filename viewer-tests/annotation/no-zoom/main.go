// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

// This program generates a PDF file for checking how a viewer handles
// annotations which keep their size whatever the magnification: those flagged
// NoZoom (PDF 32000-2 §12.5.3) and text annotations, which carry the flag
// implicitly (§12.5.6.4).
//
// Such an annotation is drawn at its natural size and positioned so that the
// upper-left corner of its rectangle in default user space does not move.
// Every annotation here is paired with a cross drawn in the page content at
// exactly that point, so at any magnification the annotation's corner mark
// should sit on the cross.
//
// The layout is repeated once per value of /Rotate, which separates NoZoom
// from NoRotate (§12.5.3).  A NoZoom annotation turns with the page, so the
// anchored corner moves to a different corner of the box on screen under each
// rotation; a NoRotate one stays upright and keeps it at the top left
// throughout.  Text annotations carry both flags, so each page includes an
// explicitly NoRotate box as their control.
package main

import (
	"fmt"
	"log"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/page"
)

const (
	// The boxes are 60×30 pt, which at 100% is their size on screen too.
	// The wide shape makes it plain when an appearance has been stretched to
	// fit a rectangle of the wrong proportions.
	boxWidth  = 60.0
	boxHeight = 30.0

	labelX = 60.0
	crossR = 5.0
)

var (
	fixedColor  = color.DeviceRGB{0.85, 0.25, 0.15}
	scaledColor = color.DeviceRGB{0.25, 0.45, 0.80}
	crossColor  = color.DeviceGray(0.2)
	guideColor  = color.DeviceGray(0.75)
)

// frame maps the coordinates a page is laid out in — the upright view the
// reader sees — to the page's own coordinates, which a /Rotate entry turns
// before display.  The layout code works in the former, annotation rectangles
// live in the latter.
//
// Writing D for the display map, the four rotations give
//
//	quarter 0:  D(X, Y) = (X, Y)
//	quarter 1:  D(X, Y) = (Y, pageW-X)
//	quarter 2:  D(X, Y) = (pageW-X, pageH-Y)
//	quarter 3:  D(X, Y) = (pageH-Y, X)
//
// [frame.point] is D and [frame.matrix] is its inverse.
type frame struct {
	// quarter is the number of clockwise quarter turns the viewer applies
	// on display, 0 to 3.
	quarter int
	// width and height of the page in its own coordinates
	pageW, pageH float64
}

// newFrame builds the frame of a page with the given rotation.
//
// The rotation must be resolved: [page.RotateInherit] takes its value from the
// parent Pages node, which a frame cannot see, so none of the maps below can
// be worked out for it.  It is also the zero value of [page.Rotation], so
// rejecting it keeps a frame from being built without one.
func newFrame(rotate page.Rotation, pageW, pageH float64) frame {
	if rotate == page.RotateInherit {
		panic("newFrame needs a resolved page rotation")
	}
	return frame{quarter: rotate.Degrees() / 90, pageW: pageW, pageH: pageH}
}

// height is the height of the upright view, which the layout measures from.
// A quarter or three-quarter turn swaps the page's two dimensions.
func (f frame) height() float64 {
	if f.quarter%2 == 1 {
		return f.pageW
	}
	return f.pageH
}

// matrix maps the upright view onto the page, for the page content.
func (f frame) matrix() matrix.Matrix {
	switch f.quarter {
	case 1:
		return matrix.Matrix{0, 1, -1, 0, f.pageW, 0}
	case 2:
		return matrix.Matrix{-1, 0, 0, -1, f.pageW, f.pageH}
	case 3:
		return matrix.Matrix{0, -1, 1, 0, 0, f.pageH}
	}
	return matrix.Identity
}

// point maps a point of the page's own coordinates back to the upright view,
// for marking where a corner of an annotation rectangle ends up on screen.
func (f frame) point(x, y float64) (u, v float64) {
	switch f.quarter {
	case 1:
		return y, f.pageW - x
	case 2:
		return f.pageW - x, f.pageH - y
	case 3:
		return f.pageH - y, x
	}
	return x, y
}

// rotation is the turn [frame.matrix] applies, without its shift.  It serves
// as the Matrix entry of an appearance which turns with the page: the §12.5.5
// algorithm takes the bounding box through Matrix before fitting it to the
// annotation rectangle, so an appearance laid out in the upright view needs
// this turn to end up with the proportions of the rectangle [frame.rect]
// builds.  Without it a quarter turn would leave the two transposed and the
// fit would stretch the appearance.
func (f frame) rotation() matrix.Matrix {
	switch f.quarter {
	case 1:
		return matrix.Matrix{0, 1, -1, 0, 0, 0}
	case 2:
		return matrix.Matrix{-1, 0, 0, -1, 0, 0}
	case 3:
		return matrix.Matrix{0, -1, 1, 0, 0, 0}
	}
	return matrix.Identity
}

// rect maps a rectangle of the upright view to an annotation rectangle in the
// page's own coordinates.  A quarter turn keeps rectangles axis-aligned, so
// the result is exact.
func (f frame) rect(u, v, w, h float64) pdf.Rectangle {
	switch f.quarter {
	case 1:
		return pdf.Rectangle{LLx: f.pageW - v - h, LLy: u, URx: f.pageW - v, URy: u + w}
	case 2:
		return pdf.Rectangle{
			LLx: f.pageW - u - w, LLy: f.pageH - v - h,
			URx: f.pageW - u, URy: f.pageH - v,
		}
	case 3:
		return pdf.Rectangle{LLx: v, LLy: f.pageH - u - w, URx: v + h, URy: f.pageH - u}
	}
	return pdf.Rectangle{LLx: u, LLy: v, URx: u + w, URy: v + h}
}

// anchorRect returns an annotation rectangle of the given size in the page's
// own coordinates, anchored at the point of the upright view which (u, v)
// names.  It is the counterpart of [frame.rect] for an annotation flagged
// NoRotate: such an annotation is drawn in the orientation of the page's own
// coordinates, pivoted about the upper-left corner of its rectangle
// (§12.5.3), so its rectangle must have the proportions the appearance is
// drawn in rather than those of the slot it fills on screen.  The box then
// hangs down and to the right of (u, v) on screen, like every other box.
func (f frame) anchorRect(u, v, w, h float64) pdf.Rectangle {
	p := f.matrix().Apply(vec.Vec2{X: u, Y: v})
	return pdf.Rectangle{LLx: p.X, LLy: p.Y - h, URx: p.X + w, URy: p.Y}
}

// corner names a corner of a box.  The four are listed clockwise from the top
// left, so that adding a number of quarter turns to one gives the corner it
// turns into.
type corner int

const (
	topLeft corner = iota
	topRight
	bottomRight
	bottomLeft
)

func (c corner) String() string {
	return [...]string{"top left", "top right", "bottom right", "bottom left"}[c]
}

// anchorCorner is the corner of the on-screen box which the anchor lands on
// once an annotation has turned with the page.  Since [frame.rotation] leaves
// such an appearance upright on screen, it also names the corner of the
// appearance which the notch belongs in.
func (f frame) anchorCorner() corner {
	return corner(f.quarter)
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// PDF 1.7, so the text annotations can be written without an appearance
	// stream and each viewer draws its own icon: that is the everyday case of
	// an annotation which holds its size.  PDF 2.0 requires one for every
	// annotation except Popup, Projection and Link.
	doc, err := document.CreateMultiPage("test.pdf", document.A4, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	F := font.Must(standard.Helvetica.New())
	B := font.Must(standard.HelveticaBold.New())

	// one page per rotation: the anchored corner lands somewhere different
	// on screen in each
	for _, rotate := range []page.Rotation{
		page.Rotate0, page.Rotate90, page.Rotate180, page.Rotate270,
	} {
		if err := writePage(doc, F, B, rotate); err != nil {
			return err
		}
	}

	return doc.Close()
}

func writePage(doc *document.MultiPage, F, B font.Instance, rotate page.Rotation) error {
	p := doc.AddPage()
	p.Page.Rotate = rotate

	f := newFrame(rotate, document.A4.URx, document.A4.URy)
	h := f.height()

	// lay the page out as the reader sees it; the /Rotate entry turns it back
	p.Transform(f.matrix())

	heading := "Annotations which hold their size"
	if deg := rotate.Degrees(); deg != 0 {
		heading += fmt.Sprintf(" (page rotated %d°)", deg)
	}
	p.TextBegin()
	p.TextSetFont(B, 16)
	p.TextFirstLine(labelX, h-62)
	p.TextShow(heading)
	p.TextEnd()

	intro := []string{
		"Zoom in and out.  The red boxes should keep their size on screen; the blue one should grow",
		"and shrink with the page.  Each box has a notch cut out of one corner, marking the upper-left",
		"corner of its rectangle in the page's own coordinates.  The notch should sit on the black cross",
		"beside it at every magnification, and each icon below should sit on its cross the same way.",
	}
	if rotate.Degrees() != 0 {
		intro = append(intro,
			"",
			"The rotation separates two rules.  A NoZoom box turns with the page, which carries its notch",
			fmt.Sprintf("and its cross to the %s corner of the box on screen.  A NoRotate box stays upright,", f.anchorCorner()),
			"so both stay at its top left; the text annotations carry NoRotate as well and do the same.",
		)
	}

	const introLeading = 13.0
	introTop := h - 86
	p.TextBegin()
	p.TextSetFont(F, 10)
	p.TextFirstLine(labelX, introTop)
	for i, line := range intro {
		if i > 0 {
			p.TextSecondLine(0, -introLeading)
		}
		p.TextShow(line)
	}
	p.TextEnd()

	// the rows start a fixed distance below the last line of the intro, which
	// is longer on the rotated pages
	y := introTop - introLeading*float64(len(intro)-1) - 50

	// A NoZoom annotation and, beside it, the same appearance without the
	// flag: the pair reads as a single "does this scale?" comparison.
	addBox(p, f, F, B, box{
		x: labelX, y: y, w: boxWidth, h: boxHeight,
		name: "NoZoom", note: "holds its size",
		fill: fixedColor, flags: annotation.FlagNoZoom,
	})
	addBox(p, f, F, B, box{
		x: labelX + 260, y: y, w: boxWidth, h: boxHeight,
		name: "plain", note: "scales with the page",
		fill: scaledColor,
	})
	y -= 90

	// A link is not a markup annotation, so it survives a viewer's
	// hide-markup setting; a square does not.
	addBox(p, f, F, B, box{
		x: labelX, y: y, w: boxWidth, h: boxHeight,
		name: "NoZoom link", note: "survives hide-markup",
		fill: fixedColor, flags: annotation.FlagNoZoom, asLink: true,
	})

	// The control for the text annotations below, which carry NoRotate
	// implicitly.  On an unrotated page the flag does nothing.
	noRotateNote := "also NoZoom: no different here"
	if rotate.Degrees() != 0 {
		noRotateNote = "also NoZoom: stays upright"
	}
	addBox(p, f, F, B, box{
		x: labelX + 260, y: y, w: boxWidth, h: boxHeight,
		name: "NoRotate", note: noRotateNote,
		fill:  fixedColor,
		flags: annotation.FlagNoZoom | annotation.FlagNoRotate,
	})

	y -= 90

	// text annotations carry the NoZoom flag implicitly
	p.TextBegin()
	p.TextSetFont(B, 10)
	p.TextSetMatrix(matrix.Translate(labelX, y))
	p.TextShow("Text annotations")
	p.TextSetFont(F, 8)
	p.TextSetMatrix(matrix.Translate(labelX, y-13))
	p.TextShow("No flag of their own: the size is fixed by the annotation type (§12.5.6.4).")
	p.TextEnd()

	icons := []annotation.TextIcon{
		annotation.TextIconComment,
		annotation.TextIconNote,
		annotation.TextIconKey,
		annotation.TextIconHelp,
		annotation.TextIconParagraph,
	}
	iconY := y - 60
	for i, icon := range icons {
		x := labelX + float64(i)*80

		// NoRotate is implicit here, so the rectangle is anchored the same way
		// as the NoRotate box above
		rect := f.anchorRect(x, iconY+20, 20, 20)
		p.Page.Annots = append(p.Page.Annots, &annotation.Text{
			Common: annotation.Common{
				Rect:     rect,
				Flags:    annotation.FlagPrint,
				Contents: string(icon),
			},
			Icon: icon,
		})
		u, v := f.point(rect.LLx, rect.URy)
		drawCross(p, u, v)

		// clear of the icon under every rotation: a viewer draws it at its own
		// size, hanging down from the anchor, so it can reach a whole icon
		// height below the rectangle
		p.TextBegin()
		p.TextSetFont(F, 8)
		p.TextSetMatrix(matrix.Translate(x, iconY-38))
		p.TextShow(string(icon))
		p.TextEnd()
	}

	return p.Close()
}

// box is one labelled box annotation.  A zero flags value gives the annotation
// which scales with the page, for comparison.
type box struct {
	x, y   float64 // upper-left corner in the upright view
	w, h   float64
	name   string
	note   string
	fill   color.Color
	flags  annotation.Flags
	asLink bool // a link rather than a square, to survive hide-markup
}

// addBox places one box annotation with its anchor cross and label.
func addBox(p *document.Page, f frame, F, B font.Instance, b box) {
	var rect pdf.Rectangle
	var ap *form.Form
	if b.flags&annotation.FlagNoRotate != 0 {
		// drawn in the orientation of the page's own coordinates, so the
		// rectangle is built in that orientation too
		rect = f.anchorRect(b.x, b.y, b.w, b.h)
		ap = boxForm(F, b.fill, b.name, b.w, b.h, matrix.Identity, topLeft)
	} else {
		// turns with the page: the rectangle of the slot on screen, with the
		// turn carried by the appearance matrix
		rect = f.rect(b.x, b.y-b.h, b.w, b.h)
		ap = boxForm(F, b.fill, b.name, b.w, b.h, f.rotation(), f.anchorCorner())
	}

	common := annotation.Common{
		Rect:       rect,
		Flags:      annotation.FlagPrint | b.flags,
		Appearance: &appearance.Dict{Normal: ap},
	}

	if b.asLink {
		p.Page.Annots = append(p.Page.Annots, &annotation.Link{Common: common})
	} else {
		p.Page.Annots = append(p.Page.Annots, &annotation.Square{Common: common})
	}

	// the anchor is the upper-left corner of the rectangle in the page's own
	// coordinates; which corner of the box that is on screen depends on the
	// rotation, unless the annotation is flagged NoRotate
	u, v := f.point(rect.LLx, rect.URy)
	drawCross(p, u, v)

	// the note clears the anchor cross, which reaches crossR above the top edge
	p.TextBegin()
	p.TextSetFont(B, 10)
	p.TextSetMatrix(matrix.Translate(b.x, b.y+21))
	p.TextShow(b.name)
	p.TextSetFont(F, 8)
	p.TextSetMatrix(matrix.Translate(b.x, b.y+10))
	p.TextShow(b.note)
	p.TextEnd()
}

// drawCross marks the anchor point in the page content, so that it scales
// with the page and the annotation can be seen holding still against it.
func drawCross(p *document.Page, u, v float64) {
	p.PushGraphicsState()
	p.SetStrokeColor(crossColor)
	p.SetLineWidth(0.5)
	p.MoveTo(u-crossR, v)
	p.LineTo(u+crossR, v)
	p.MoveTo(u, v-crossR)
	p.LineTo(u, v+crossR)
	p.Stroke()
	p.PopGraphicsState()
}

// boxForm builds the appearance: a filled w×h box with one corner notched, so
// that the corner the annotation is anchored by can be seen.  The box is laid
// out upright and m turns it into the orientation of the annotation
// rectangle, which for a box that turns with the page is the turn the page
// itself applies.
//
// The notch is a right isosceles wedge and the label is centred, so a fit to
// a rectangle of the wrong proportions shows up as a leaning notch and
// squeezed lettering.
func boxForm(F font.Instance, fill color.Color, label string, w, h float64, m matrix.Matrix, notch corner) *form.Form {
	b := builder.New(content.Form, nil, pdf.V1_7)

	b.SetFillColor(fill)
	b.Rectangle(0, 0, w, h)
	b.Fill()

	const notchSide = 10.0
	x, y := 0.0, h // the notch corner, and the two directions away from it
	dx, dy := 1.0, -1.0
	if notch == topRight || notch == bottomRight {
		x, dx = w, -1
	}
	if notch == bottomRight || notch == bottomLeft {
		y, dy = 0, 1
	}
	b.SetFillColor(color.DeviceGray(1))
	b.MoveTo(x, y)
	b.LineTo(x+dx*notchSide, y)
	b.LineTo(x, y+dy*notchSide)
	b.ClosePath()
	b.Fill()

	b.SetStrokeColor(guideColor)
	b.SetLineWidth(0.5)
	b.Rectangle(0.25, 0.25, w-0.5, h-0.5)
	b.Stroke()

	b.SetFillColor(color.DeviceGray(1))
	b.TextBegin()
	b.TextSetFont(F, 7)
	b.TextFirstLine(0, h/2-2.5)
	b.TextShowAligned(label, w, 0.5)
	b.TextEnd()

	return &form.Form{
		Content: builder.Must(b.Harvest()),
		Res:     b.Resources,
		BBox:    pdf.Rectangle{URx: w, URy: h},
		Matrix:  m,
	}
}
