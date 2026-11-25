// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// This program shows redaction annotations with fallback appearances.  The
// normal appearance marks the region as pending (outline and light tint).
// Hovering over or pressing an annotation previews the applied redaction in
// viewers which draw the rollover and down appearances.
package main

import (
	"fmt"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
)

const margin = 60.0

var alignName = map[pdf.TextAlign]string{
	pdf.TextAlignLeft:   "left",
	pdf.TextAlignCenter: "centre",
	pdf.TextAlignRight:  "right",
}

// outline is the border a reader supplies for a file without /Border
var outline = &annotation.Border{Width: 1}

func main() {
	fmt.Println("writing test.pdf ...")
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	opt := &pdf.WriterOptions{HumanReadable: true}
	doc, err := document.CreateMultiPage("test.pdf", document.A4, pdf.V1_7, opt)
	if err != nil {
		return err
	}
	gen, err := fallback.NewStyle().New(pdf.V1_7)
	if err != nil {
		return err
	}
	F := font.Must(standard.Helvetica.New())

	page := doc.AddPage()
	y := document.A4.URy - margin
	// down moves y down the page, rounded so that coordinates stay short
	down := func(d float64) { y = pdf.Round(y-d, 2) }
	label := func(s string) {
		down(26)
		page.TextBegin()
		page.TextSetFont(F, 9)
		page.TextSetMatrix(matrix.Translate(margin, y))
		page.TextShow(s)
		page.TextEnd()
		down(6)
	}
	add := func(a *annotation.Redact) error {
		if err := gen.AddAppearance(a); err != nil {
			return err
		}
		page.Page.AddAnnots(a)
		return nil
	}
	box := func(h float64) pdf.Rectangle {
		down(h)
		r := pdf.Rectangle{LLx: margin, LLy: y, URx: margin + 300, URy: y + h}
		down(6)
		return r
	}

	// text with quads over some words
	const line = "The password is hunter2 and the account number is 12345678."
	const size = 12.0
	label("quads over the words \"hunter2\" and \"12345678\"")
	down(size + 2)
	baseline := y
	page.TextBegin()
	page.TextSetFont(F, size)
	page.TextSetMatrix(matrix.Translate(margin, baseline))
	page.TextShow(line)
	page.TextEnd()
	width := func(s string) float64 { return F.Layout(nil, size, s).TotalWidth() }
	quad := func(prefix, word string) []vec.Vec2 {
		x0 := pdf.Round(margin+width(prefix), 2)
		x1 := pdf.Round(margin+width(prefix+word), 2)
		top := pdf.Round(baseline+F.GetGeometry().Ascent*size, 2)
		bot := pdf.Round(baseline+F.GetGeometry().Descent*size, 2)
		return []vec.Vec2{{X: x0, Y: top}, {X: x1, Y: top}, {X: x0, Y: bot}, {X: x1, Y: bot}}
	}
	q := append(quad("The password is ", "hunter2"), quad("The password is hunter2 and the account number is ", "12345678")...)
	r := pdf.RectangleFromPoints(q...)
	if err := add(&annotation.Redact{
		Rect: r, Flags: annotation.FlagPrint, Border: outline,
		QuadPoints: q,
		FillColor:  color.DeviceRGB{0, 0, 0},
	}); err != nil {
		return err
	}
	y = pdf.Round(baseline-10, 2)

	label("rectangle only")
	if err := add(&annotation.Redact{
		Rect: box(30), Flags: annotation.FlagPrint, Border: outline,
	}); err != nil {
		return err
	}

	label("fill colour")
	if err := add(&annotation.Redact{
		Rect: box(30), Flags: annotation.FlagPrint, Border: outline,
		FillColor: color.DeviceRGB{0.2, 0.2, 0.6},
	}); err != nil {
		return err
	}

	for _, align := range []pdf.TextAlign{pdf.TextAlignLeft, pdf.TextAlignCenter, pdf.TextAlignRight} {
		label(fmt.Sprintf("overlay text, %s aligned, fixed size", alignName[align]))
		if err := add(&annotation.Redact{
			Rect: box(30), Flags: annotation.FlagPrint, Border: outline,
			FillColor:         color.DeviceRGB{0, 0, 0},
			OverlayText:       pdf.Text("REDACTED"),
			DefaultAppearance: "/Helv 12 Tf 1 g",
			Align:             align,
		}); err != nil {
			return err
		}
	}

	label("overlay text, automatic size")
	if err := add(&annotation.Redact{
		Rect: box(30), Flags: annotation.FlagPrint, Border: outline,
		FillColor:         color.DeviceRGB{0.9, 0.9, 0.9},
		OverlayText:       pdf.Text("SECRET"),
		DefaultAppearance: "/Helv 0 Tf 0.8 0 0 rg",
		Align:             pdf.TextAlignCenter,
	}); err != nil {
		return err
	}

	for _, align := range []pdf.TextAlign{pdf.TextAlignLeft, pdf.TextAlignCenter} {
		label(fmt.Sprintf("repeated overlay text, %s aligned", alignName[align]))
		if err := add(&annotation.Redact{
			Rect: box(60), Flags: annotation.FlagPrint, Border: outline,
			FillColor:         color.DeviceRGB{0, 0, 0},
			OverlayText:       pdf.Text("CONFIDENTIAL"),
			DefaultAppearance: "/Helv 9 Tf 1 1 0 rg",
			Repeat:            true,
			Align:             align,
		}); err != nil {
			return err
		}
	}

	// overlay form
	b := builder.New(content.Form, nil, pdf.V1_7)
	b.SetFillColor(color.DeviceRGB{0, 0.5, 0})
	b.Rectangle(0, 0, 300, 30)
	b.Fill()
	b.SetStrokeColor(color.DeviceGray(1))
	b.SetLineWidth(2)
	b.MoveTo(0, 0)
	b.LineTo(300, 30)
	b.MoveTo(0, 30)
	b.LineTo(300, 0)
	b.Stroke()
	ops, err := b.Harvest()
	if err != nil {
		return err
	}
	overlay := &form.Form{Content: ops, Res: b.Resources, BBox: pdf.Rectangle{URx: 300, URy: 30}}
	label("overlay form")
	if err := add(&annotation.Redact{
		Rect: box(30), Flags: annotation.FlagPrint, Border: outline,
		Overlay: overlay,
	}); err != nil {
		return err
	}

	if err := page.Close(); err != nil {
		return err
	}
	return doc.Close()
}
