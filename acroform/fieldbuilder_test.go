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

package acroform

import (
	"testing"

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
	btn.Ff = optional.New(FieldRadio)
	sub := btn.NewField("option1")

	if got := ResolvedFT(sub); got != "Btn" {
		t.Errorf("ResolvedFT = %q, want %q", got, "Btn")
	}
	if got := ResolvedFf(sub); got != FieldRadio {
		t.Errorf("ResolvedFf = %d, want %d", got, FieldRadio)
	}
}

// calling a builder method on a field assembled from a raw struct literal is a
// programming error and panics
func TestBuilderPanicOnLiteral(t *testing.T) {
	assertPanics(t, "NewTextField on literal", func() {
		(&FieldCommon{}).NewTextField("x")
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
