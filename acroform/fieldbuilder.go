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

// A field tree is plain top-down data: assemble it with [Group] literals and
// the field constructors below, placing children directly in a parent's Kids or
// in [InteractiveForm.Fields]. There are no parent links to wire; the encoder
// derives them from the tree. Attach a field's on-page appearance with
// "seehuhn.de/go/pdf/annotation".AddWidget.

// NewTextField returns a new text field (field type "Tx") with the given
// partial name.
func NewTextField(name string) *TextField {
	f := &TextField{}
	f.Name = name
	return f
}

// NewButtonField returns a new button field (field type "Btn") with the given
// partial name. Set its flags to select a check box, radio button, or push
// button.
func NewButtonField(name string) *ButtonField {
	f := &ButtonField{}
	f.Name = name
	return f
}

// NewChoiceField returns a new choice field (field type "Ch") with the given
// partial name.
func NewChoiceField(name string) *ChoiceField {
	f := &ChoiceField{}
	f.Name = name
	return f
}

// NewSignatureField returns a new signature field (field type "Sig") with the
// given partial name.
func NewSignatureField(name string) *SignatureField {
	f := &SignatureField{}
	f.Name = name
	return f
}
