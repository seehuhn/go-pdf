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

package decode

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/optional"
)

// a terminal field built with several widgets keeps them all and round-trips
func TestBuilderMultiWidgetRoundTrip(t *testing.T) {
	for _, version := range testVersions {
		t.Run(version.String(), func(t *testing.T) {
			btn := (&acroform.InteractiveForm{}).NewButtonField("color")
			btn.Ff = optional.New(acroform.FieldRadio)
			btn.Opt = []string{"red", "green"}
			annotation.AddWidget(btn, pdf.Rectangle{URx: 20, URy: 20})
			annotation.AddWidget(btn, pdf.Rectangle{LLx: 30, URx: 50, URy: 20})

			if n := len(btn.Kids); n != 2 {
				t.Fatalf("field has %d kids, want 2", n)
			}
			fieldRoundTripTest(t, version, btn)
		})
	}
}

// a builder-assembled hierarchy of non-terminal and terminal fields round-trips
func TestBuilderTreeRoundTrip(t *testing.T) {
	form := &acroform.InteractiveForm{}
	root := form.NewField("request")
	text := root.NewTextField("text")
	text.V = pdf.String("hello")
	root.NewButtonField("flag")

	formRoundTripTest(t, pdf.V1_7, form)
}
