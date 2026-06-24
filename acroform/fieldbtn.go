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

import "seehuhn.de/go/pdf"

// PDF 2.0 sections: 12.7.5.2.2 12.7.5.2.3 12.7.5.2.4

// ButtonVariant identifies the kind of a button form field ([ButtonField]).
type ButtonVariant int

const (
	// ButtonCheckbox is a check box: a button with neither the [FieldRadio] nor
	// the [FieldPushbutton] flag set.
	ButtonCheckbox ButtonVariant = iota

	// ButtonRadio is a set of radio buttons: a button with the [FieldRadio] flag
	// set.
	ButtonRadio

	// ButtonPush is a push button: a button with the [FieldPushbutton] flag set.
	// A push button retains no value.
	ButtonPush
)

// ButtonField is a button form field (field type "Btn").
//
// The three button kinds — check box, radio button, and push button — share one
// dictionary layout and differ only in their flags, so a single type represents
// all of them. Use [ButtonField.Variant] to obtain the kind.
type ButtonField struct {
	Common

	VariableText

	// Opt (optional) holds the export value of each of the field's widget
	// annotations, in the same order as the field's widgets (see
	// [Field.Widgets]). It allows widgets that share an on-state name to be told
	// apart. It does not apply to push buttons.
	//
	// This corresponds to the /Opt entry in the PDF field dictionary.
	Opt []string

	// V (optional) is the field's value, the on-state name of the selected
	// appearance ("Off" for none) of a check box or radio button. A push button
	// retains no value, so V is empty for that variant. An empty value indicates
	// that the field has no value of its own.
	//
	// This corresponds to the /V entry in the PDF field dictionary.
	V pdf.Name

	// DV (optional) is the field's default value, used when the form is reset.
	//
	// This corresponds to the /DV entry in the PDF field dictionary.
	DV pdf.Name
}

var _ Field = (*ButtonField)(nil)

// Variant reports whether the button is a check box, radio button, or push
// button, derived from the field's flags.
func (f *ButtonField) Variant() ButtonVariant {
	switch {
	case f.Flags&FieldPushbutton != 0:
		return ButtonPush
	case f.Flags&FieldRadio != 0:
		return ButtonRadio
	default:
		return ButtonCheckbox
	}
}

// FieldType implements the [Field] interface.
func (f *ButtonField) FieldType() pdf.Name { return "Btn" }

func (f *ButtonField) fillDict(rm *pdf.ResourceManager, dict pdf.Dict) error {
	if err := f.VariableText.fillVarTextDict(rm, dict); err != nil {
		return err
	}
	if len(f.Opt) > 0 {
		if err := pdf.CheckVersion(rm.Out, "button field Opt entry", pdf.V1_4); err != nil {
			return err
		}
		arr := make(pdf.Array, len(f.Opt))
		for i, s := range f.Opt {
			arr[i] = pdf.TextString(s)
		}
		dict["Opt"] = arr
	}
	fillName(dict, "V", f.V)
	fillName(dict, "DV", f.DV)
	return nil
}

// fillName writes a name entry, omitting it when the name is empty.
func fillName(dict pdf.Dict, key pdf.Name, n pdf.Name) {
	if n != "" {
		dict[key] = n
	}
}
