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

// Command media-form-clip writes test.pdf, a page with five Screen
// annotations whose rendition actions play a form XObject as the media clip
// (§13.2.4.2).  The form has a 2:1 bounding box and shows an oriented
// picture, so that the fit mode of each screen (§13.2.4.3) can be read off
// the result.
package main

import (
	"fmt"
	"math"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/media"
	"seehuhn.de/go/pdf/optional"
)

const (
	screenW = 140.0
	screenH = 100.0
	gap     = 20.0
	screenY = 200.0

	// the clip form has a 2:1 bounding box
	clipW = 200.0
	clipH = 100.0
)

var variants = []struct {
	label  string
	fit    media.FitMode
	rotate float64 // degrees, form matrix
	timed  bool
}{
	{"meet", media.FitMeet, 0, true},
	{"slice", media.FitSlice, 0, false},
	{"fill", media.FitFill, 30, false},
	{"hidden", media.FitHidden, 0, false},
	{"scroll", media.FitScroll, 0, false},
}

func main() {
	if err := createDocument("test.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	paper := document.A4r
	opt := &pdf.WriterOptions{HumanReadable: true}
	doc, err := document.CreateSinglePage(filename, paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}
	bold := font.Must(standard.HelveticaBold.New())
	body := font.Must(standard.Helvetica.New())

	doc.TextBegin()
	doc.TextSetFont(bold, 16)
	doc.TextSetMatrix(matrix.Translate(0, 520))
	doc.TextShowAligned("Media clip given as a form XObject", paper.URx, 0.5)
	doc.TextSetFont(body, 10)
	for i, line := range caption {
		doc.TextSetMatrix(matrix.Translate(0, 490-float64(i)*14))
		doc.TextShowAligned(line, paper.URx, 0.5)
	}
	doc.TextEnd()

	x0 := pdf.Round((paper.URx-5*screenW-4*gap)/2, 2)
	for i, v := range variants {
		x := pdf.Round(x0+float64(i)*(screenW+gap), 2)
		rect := pdf.Rectangle{LLx: x, LLy: screenY, URx: x + screenW, URy: screenY + screenH}

		app, err := appearanceForm(body, v.label)
		if err != nil {
			return err
		}
		screen := &annotation.Screen{
			Rect:       rect,
			Contents:   "Screen, fit mode " + v.label,
			Flags:      annotation.FlagPrint,
			Appearance: &appearance.Dict{Normal: app},
			Title:      v.label,
		}
		ref := doc.RM.GetReference(screen)

		clip, err := clipForm(bold, v.rotate)
		if err != nil {
			return err
		}
		entries := &media.MediaPlayEntries{Fit: v.fit}
		if v.timed {
			entries.Duration = &media.MediaDuration{
				Kind: media.DurationExplicit,
				Time: &media.Timespan{Seconds: 3},
			}
		}
		screen.Action = &action.Rendition{
			R: &media.MediaRendition{
				Name: "form " + v.label,
				Clip: &media.MediaClipData{DataForm: clip},
				Play: &media.MediaPlayParameters{MustHonour: entries},
			},
			AN: ref,
			OP: optional.NewUInt(0),
		}
		doc.Page.AddAnnots(screen)
	}

	return doc.Close()
}

var caption = []string{
	"Click each grey frame.  The viewer should play the same 2:1 form XObject in the screen's 140 x 100 pt rectangle.",
	"The form shows a filled ellipse with a thick arrow pointing up and to the right, inside a black border at the edge of the BBox.",
	"meet: the whole form is visible, scaled uniformly, with bars at top and bottom.  slice: uniformly scaled, the rectangle is filled, the sides are cut off.",
	"fill: the form is stretched to the rectangle (the arrow is distorted); this form also has a 30 degree rotation in its Matrix.",
	"hidden: unscaled, clipped to the rectangle.  scroll: unscaled, with a scrolling interface.  meet also has a 3 second play duration.",
	"Viewers which do not support form XObjects as media clips will show only the grey frames.",
}

// appearanceForm draws the normal appearance of a screen annotation.
func appearanceForm(f font.Instance, label string) (*form.Form, error) {
	b := builder.New(content.Form, nil, pdf.V1_7)
	b.SetStrokeColor(color.DeviceGray(0.6))
	b.SetLineWidth(0.5)
	b.Rectangle(0.25, 0.25, screenW-0.5, screenH-0.5)
	b.Stroke()
	b.SetFillColor(color.DeviceGray(0.3))
	b.TextBegin()
	b.TextSetFont(f, 9)
	b.TextFirstLine(0, screenH/2+2)
	b.TextShowAligned(label, screenW, 0.5)
	b.TextFirstLine(0, -12)
	b.TextShowAligned("click to show", screenW, 0.5)
	b.TextEnd()
	ops, err := b.Harvest()
	if err != nil {
		return nil, err
	}
	return &form.Form{
		Content: ops,
		Res:     b.Resources,
		BBox:    pdf.Rectangle{URx: screenW, URy: screenH},
	}, nil
}

// clipForm draws the media clip: a 2:1 picture with an orientation.
func clipForm(f font.Instance, rotate float64) (*form.Form, error) {
	b := builder.New(content.Form, nil, pdf.V1_7)

	// filled ellipse
	const cx, cy, rx, ry = clipW / 2, clipH / 2, 80.0, 38.0
	const k = 0.5523
	kx, ky := pdf.Round(k*rx, 2), pdf.Round(k*ry, 2)
	b.SetFillColor(color.DeviceRGB{0.75, 0.85, 1})
	b.MoveTo(cx+rx, cy)
	b.CurveTo(cx+rx, cy+ky, cx+kx, cy+ry, cx, cy+ry)
	b.CurveTo(cx-kx, cy+ry, cx-rx, cy+ky, cx-rx, cy)
	b.CurveTo(cx-rx, cy-ky, cx-kx, cy-ry, cx, cy-ry)
	b.CurveTo(cx+kx, cy-ry, cx+rx, cy-ky, cx+rx, cy)
	b.Fill()

	// thick arrow pointing up-right, from the lower left of the ellipse
	const ang = math.Pi / 4
	const length, shaft, head, headLen = 90.0, 10.0, 26.0, 28.0
	rot := matrix.Rotate(ang)
	pts := [][2]float64{
		{0, -shaft / 2}, {length - headLen, -shaft / 2}, {length - headLen, -head / 2},
		{length, 0},
		{length - headLen, head / 2}, {length - headLen, shaft / 2}, {0, shaft / 2},
	}
	b.SetFillColor(color.DeviceRGB{0.8, 0.1, 0.1})
	for i, p := range pts {
		v := rot.Apply(vec.Vec2{X: p[0], Y: p[1]})
		x, y := pdf.Round(cx-length*math.Cos(ang)/2+v.X, 2), pdf.Round(cy-length*math.Sin(ang)/2+v.Y, 2)
		if i == 0 {
			b.MoveTo(x, y)
		} else {
			b.LineTo(x, y)
		}
	}
	b.ClosePath()
	b.Fill()

	// border around the BBox
	b.SetStrokeColor(color.Black)
	b.SetLineWidth(4)
	b.Rectangle(2, 2, clipW-4, clipH-4)
	b.Stroke()

	b.SetFillColor(color.Black)
	b.TextBegin()
	b.TextSetFont(f, 12)
	b.TextFirstLine(8, 8)
	b.TextShow("origin")
	b.TextEnd()

	ops, err := b.Harvest()
	if err != nil {
		return nil, err
	}
	fm := &form.Form{
		Content: ops,
		Res:     b.Resources,
		BBox:    pdf.Rectangle{URx: clipW, URy: clipH},
	}
	if rotate != 0 {
		a := rotate * math.Pi / 180
		fm.Matrix = matrix.Matrix{
			pdf.Round(math.Cos(a), 4), pdf.Round(math.Sin(a), 4),
			pdf.Round(-math.Sin(a), 4), pdf.Round(math.Cos(a), 4),
			0, 0,
		}
	}
	return fm, nil
}
