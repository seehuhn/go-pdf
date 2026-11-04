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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/text"
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

const (
	left      = 50.0
	textWidth = 420.0
	boxW      = 90.0
	boxH      = 70.0
	boxGap    = 20.0
)

func run() error {
	paper := &pdf.Rectangle{URx: 2*left + textWidth, URy: 465}
	opt := &pdf.WriterOptions{HumanReadable: true}
	page, err := document.CreateSinglePage("test.pdf", paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	F := font.Must(standard.Helvetica.New())

	groupA := &oc.Group{Name: pdf.Text("A")}
	groupB := &oc.Group{Name: pdf.Text("B")}
	groupC := &oc.Group{Name: pdf.Text("C")}
	groupX := &oc.Group{Name: pdf.Text("X")}
	all := []*oc.Group{groupA, groupB, groupC, groupX}

	// X is the only group which starts OFF, so that a viewer which resets
	// to the default configuration before applying BaseState Unchanged
	// can be told apart from one which treats Unchanged as ON.
	props := &oc.Properties{
		OCGs: all,
		D: &oc.Configuration{
			BaseState: oc.BaseStateON,
			OFF:       []*oc.Group{groupX},
			Order:     []oc.OrderItem{groupA, groupB, groupC, groupX},
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

	black := color.DeviceGray(0)
	title := text.F{Font: F, Size: 14, Color: black}
	head := text.F{Font: F, Size: 12, Color: black}
	body := text.F{Font: F, Size: 10, Color: black}

	y := paper.URy - 35
	text.Show(page.Builder,
		text.M{X: left, Y: y},
		title, "Alternate configurations and BaseState Unchanged",
	)
	y -= 25

	// coloured boxes, each controlled by one group
	boxes := []struct {
		label string
		g     *oc.Group
		col   color.Color
	}{
		{"A", groupA, color.DeviceRGB{0.9, 0.1, 0.1}},
		{"B", groupB, color.DeviceRGB{0.1, 0.7, 0.2}},
		{"C", groupC, color.DeviceRGB{0.1, 0.2, 0.9}},
		{"X", groupX, color.DeviceGray(0.5)},
	}
	label := text.F{Font: F, Size: 36, Color: color.DeviceGray(1)}
	for i, b := range boxes {
		x := left + float64(i)*(boxW+boxGap)
		labelW := F.Layout(nil, label.Size, b.label).TotalWidth()
		page.MarkedContentStart(&graphics.MarkedContent{
			Tag:        "OC",
			Properties: b.g,
		})
		page.SetFillColor(b.col)
		page.Rectangle(x, y-boxH, boxW, boxH)
		page.Fill()
		text.Show(page.Builder,
			label,
			text.M{X: pdf.Round(x+(boxW-labelW)/2, 2), Y: y - 48},
			b.label,
		)
		page.MarkedContentEnd()
	}
	y -= boxH + 30

	text.Show(page.Builder,
		text.M{X: left, Y: y},
		body,
		text.Wrap(textWidth,
			"Groups A (red), B (green), C (blue) and X (grey) each control",
			"one box.  The default configuration turns all groups on except X,",
			"so initially the red, green and blue boxes are visible.",
		),
		text.NL,
		text.Wrap(textWidth,
			"The viewer should offer three alternate configurations:",
			"“All off”, “All on” and “Unchanged, B on, C off”",
			"(BaseState Unchanged, ON [B], OFF [C]).  Choosing “All off”",
			"should hide all four boxes and “All on” should show all four;",
			"this checks that the viewer applies configurations at all.",
			"If the viewer offers no way to choose a configuration,",
			"record that: the test below cannot be run.",
		),
		text.NL,
		head, "Test", text.NL,
		body,
		"1. Reopen the file, so that the default configuration applies.", text.NL,
		"2. In the layers panel, switch A off and X on by hand.", text.NL,
		"3. Choose “Unchanged, B on, C off”.", text.NL,
		text.NL,
		text.Wrap(textWidth,
			"Unchanged leaves A and X as the user set them, then B is turned",
			"on and C off.  The boxes visible after step 3 identify how the",
			"viewer treats BaseState Unchanged:",
		),
		text.RecordPos{UserY: &y},
	)
	y -= 10

	outcomes := []struct{ visible, meaning string }{
		{"green, grey", "correct"},
		{"red, green, grey", "Unchanged treated as ON, or not recognised (ON is the default)"},
		{"green", "Unchanged treated as OFF"},
		{"red, green", "viewer resets to the default configuration first"},
	}
	text.Show(page.Builder,
		text.M{X: left + 10, Y: y},
		head, "Visible boxes",
	)
	text.Show(page.Builder,
		text.M{X: left + 120, Y: y},
		head, "Meaning",
	)
	for _, o := range outcomes {
		y -= 15
		text.Show(page.Builder,
			text.M{X: left + 10, Y: y},
			body, o.visible,
		)
		text.Show(page.Builder,
			text.M{X: left + 120, Y: y},
			body, o.meaning,
		)
	}

	return page.Close()
}
