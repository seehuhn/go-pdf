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
	sender := NewField(root, "sender")
	name := NewField(sender, "name")
	first := NewTextField(name, "first")

	if got := first.FullyQualifiedName(); got != "request.sender.name.first" {
		t.Errorf("FullyQualifiedName = %q, want %q", got, "request.sender.name.first")
	}
	if got := ResolvedFT(first); got != "Tx" {
		t.Errorf("ResolvedFT = %q, want %q", got, "Tx")
	}
}

// a child of a typed parent inherits the parent's type and flags; the parent
// link must reach the concrete *FieldBtn rather than its embedded *FieldCommon
func TestBuilderTypedParentInheritance(t *testing.T) {
	form := &InteractiveForm{}
	btn := form.NewButtonField("color")
	btn.Ff = optional.New(FieldRadio)
	sub := NewField(btn, "option1")

	if got := ResolvedFT(sub); got != "Btn" {
		t.Errorf("ResolvedFT = %q, want %q", got, "Btn")
	}
	if got := ResolvedFf(sub); got != FieldRadio {
		t.Errorf("ResolvedFf = %d, want %d", got, FieldRadio)
	}
}

// the builder functions work on fields assembled from raw struct literals
func TestBuilderOnLiteralParent(t *testing.T) {
	btn := &FieldBtn{FieldCommon: FieldCommon{T: "color"}}
	sub := NewField(btn, "option1")

	if got := ResolvedFT(sub); got != "Btn" {
		t.Errorf("ResolvedFT = %q, want %q", got, "Btn")
	}
	if len(btn.Kids) != 1 || btn.Kids[0] != Node(sub) {
		t.Error("child was not added to the parent's Kids")
	}
}

// a nil parent yields a detached field, usable as a root field
func TestBuilderDetached(t *testing.T) {
	f := NewTextField(nil, "surname")
	if f.FieldParent() != nil {
		t.Error("detached field has a parent")
	}
	if got := f.FullyQualifiedName(); got != "surname" {
		t.Errorf("FullyQualifiedName = %q, want %q", got, "surname")
	}
}
