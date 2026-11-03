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

package main

import (
	"fmt"
	"os"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/oc"
)

// This viewer test checks how viewers apply alternate optional-content
// configurations (the Configs array of OCProperties), in particular
// BaseState Unchanged.
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	paper := &pdf.Rectangle{URx: 500, URy: 560}
	opt := &pdf.WriterOptions{HumanReadable: true}
	page, err := document.CreateSinglePage("test.pdf", paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	F := font.Must(standard.Helvetica.New())

	groupA := &oc.Group{Name: pdf.Text("A")}
	groupB := &oc.Group{Name: pdf.Text("B")}
	groupC := &oc.Group{Name: pdf.Text("C")}
	all := []*oc.Group{groupA, groupB, groupC}

	props := &oc.Properties{
		OCGs: all,
		D: &oc.Configuration{
			BaseState: oc.BaseStateON,
			Order:     []oc.OrderItem{groupA, groupB, groupC},
		},
		Configs: []*oc.Configuration{
			{
				Name:      pdf.Text("All off"),
				BaseState: oc.BaseStateOFF,
			},
			{
				Name:      pdf.Text("All on"),
				BaseState: oc.BaseStateON,
			},
			{
				Name:      pdf.Text("Unchanged, B on, C off"),
				BaseState: oc.BaseStateUnchanged,
				ON:        []*oc.Group{groupB},
				OFF:       []*oc.Group{groupC},
			},
		},
	}
	ocRef, err := page.RM.Embed(props)
	if err != nil {
		return err
	}
	page.Out.GetMeta().Catalog.OCProperties = ocRef

	const (
		left  = 50.0
		lineH = 16.0
	)
	y := 525.0

	drawText(page, F, left, y, 14, "Alternate configurations and BaseState Unchanged")
	y -= 1.8 * lineH

	// coloured boxes, each controlled by one group
	boxes := []struct {
		label string
		g     *oc.Group
		col   color.Color
	}{
		{"A", groupA, color.DeviceRGB{0.9, 0.1, 0.1}},
		{"B", groupB, color.DeviceRGB{0.1, 0.7, 0.2}},
		{"C", groupC, color.DeviceRGB{0.1, 0.2, 0.9}},
	}
	for i, b := range boxes {
		x := left + float64(i)*110
		page.MarkedContentStart(&graphics.MarkedContent{
			Tag:        "OC",
			Properties: b.g,
		})
		page.SetFillColor(b.col)
		page.Rectangle(x, y-70, 90, 70)
		page.Fill()
		page.SetFillColor(color.DeviceGray(1))
		page.TextBegin()
		page.TextSetFont(F, 36)
		page.TextFirstLine(x+34, y-50)
		page.TextShow(b.label)
		page.TextEnd()
		page.MarkedContentEnd()
	}
	y -= 70 + 2*lineH

	lines := []string{
		"Groups A (red), B (green) and C (blue) each control one box.",
		"The default configuration has BaseState ON, so all three boxes",
		"are visible initially.  The viewer should list three alternate",
		"configurations: \u201cAll off\u201d, \u201cAll on\u201d and",
		"\u201cUnchanged, B on, C off\u201d (BaseState Unchanged, ON [B], OFF [C]).",
		"",
		"Test 1",
		"1. Switch group A off by hand in the layers panel.",
		"2. Choose \u201cUnchanged, B on, C off\u201d.",
		"Expected: A stays off (Unchanged keeps the user\u2019s state),",
		"B is on, C is off.  Only the green box is visible.",
		"",
		"Test 2",
		"1. Choose \u201cAll on\u201d.  All three boxes are visible.",
		"2. Choose \u201cUnchanged, B on, C off\u201d.",
		"Expected: A stays on, B stays on, C turns off.",
		"The red and the green box are visible.",
		"",
		"A viewer which treats Unchanged as ON fails Test 1, one which",
		"treats it as OFF fails Test 2.",
	}
	for _, s := range lines {
		switch {
		case s == "":
			y -= lineH / 2
			continue
		case strings.HasPrefix(s, "Test "):
			drawText(page, F, left, y, 12, s)
		default:
			drawText(page, F, left, y, 10, s)
		}
		y -= lineH
	}

	return page.Close()
}

// drawText draws black text at the given position.
func drawText(page *document.Page, F *type1.Instance, x, y, size float64, s string) {
	page.SetFillColor(color.DeviceGray(0))
	page.TextBegin()
	page.TextSetFont(F, size)
	page.TextFirstLine(x, y)
	page.TextShow(s)
	page.TextEnd()
}

// drawGray draws gray text at the given position.
func drawGray(page *document.Page, F *type1.Instance, x, y, size float64, s string) {
	page.SetFillColor(color.DeviceGray(0.5))
	page.TextBegin()
	page.TextSetFont(F, size)
	page.TextFirstLine(x, y)
	page.TextShow(s)
	page.TextEnd()
}
