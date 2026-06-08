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

package annotation

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/optional"
)

// a builder-assembled tree resolves names and inherited attributes correctly
// without first being encoded or decoded
func TestBuilderResolveBeforeEncode(t *testing.T) {
	form := &InteractiveForm{}
	root := form.NewField("request")
	sender := root.NewField("sender")
	name := sender.NewField("name")
	first := name.NewTextField("first")

	if got := first.FullyQualifiedName(); got != "request.sender.name.first" {
		t.Errorf("FullyQualifiedName = %q, want %q", got, "request.sender.name.first")
	}
	if got := ResolvedFT(first); got != "Tx" {
		t.Errorf("ResolvedFT = %q, want %q", got, "Tx")
	}
}

// a child of a typed parent inherits the parent's type and flags; this exercises
// the self back-pointer, which must reach the outer *FieldBtn rather than its
// embedded *FieldCommon
func TestBuilderTypedParentInheritance(t *testing.T) {
	form := &InteractiveForm{}
	btn := form.NewButtonField("color")
	btn.Ff = optional.NewUInt(uint(FieldRadio))
	sub := btn.NewField("option1")

	if got := ResolvedFT(sub); got != "Btn" {
		t.Errorf("ResolvedFT = %q, want %q", got, "Btn")
	}
	if got := ResolvedFf(sub); got != FieldRadio {
		t.Errorf("ResolvedFf = %d, want %d", got, FieldRadio)
	}
}

// a terminal field built with several widgets keeps them all and round-trips
func TestBuilderMultiWidgetRoundTrip(t *testing.T) {
	for _, version := range testVersions {
		t.Run(version.String(), func(t *testing.T) {
			btn := (&InteractiveForm{}).NewButtonField("color")
			btn.Ff = optional.NewUInt(uint(FieldRadio))
			btn.Opt = []string{"red", "green"}
			btn.AddWidget(pdf.Rectangle{URx: 20, URy: 20})
			btn.AddWidget(pdf.Rectangle{LLx: 30, URx: 50, URy: 20})

			if n := len(btn.Kids); n != 2 {
				t.Fatalf("field has %d kids, want 2", n)
			}
			fieldRoundTripTest(t, version, btn)
		})
	}
}

// a builder-assembled hierarchy of non-terminal and terminal fields round-trips
func TestBuilderTreeRoundTrip(t *testing.T) {
	form := &InteractiveForm{}
	root := form.NewField("request")
	text := root.NewTextField("text")
	text.V = pdf.String("hello")
	root.NewButtonField("flag")

	formRoundTripTest(t, pdf.V1_7, form)
}

// calling a builder method on a field assembled from a raw struct literal is a
// programming error and panics
func TestBuilderPanicOnLiteral(t *testing.T) {
	assertPanics(t, "NewTextField on literal", func() {
		(&FieldCommon{}).NewTextField("x")
	})
	assertPanics(t, "AddWidget on literal", func() {
		(&FieldBtn{}).AddWidget(pdf.Rectangle{})
	})
}

func assertPanics(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected panic, got none", name)
		}
	}()
	f()
}
