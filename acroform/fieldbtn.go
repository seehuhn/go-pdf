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
	"errors"
	"fmt"
	"slices"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.7.5.2.2 12.7.5.2.3 12.7.5.2.4

// ButtonVariant identifies the kind of a [ButtonField].
type ButtonVariant int

const (
	// ButtonCheckbox is a check box: a button without the [FieldRadio] flag.
	ButtonCheckbox ButtonVariant = iota

	// ButtonRadio is a set of radio buttons: a button with the [FieldRadio] flag
	// set.
	ButtonRadio
)

// ButtonField is a button form field.
//
// This type represents a check box or a group of radio buttons.  Set the
// variant with [NewButtonField] and read it with [ButtonField.Variant].  Push
// buttons, which hold no value, are represented by [PushButtonField].  The
// field must not have the [FieldPushbutton] flag set.
//
// Use [seehuhn.de/go/pdf/annotation.AddWidget] to add a visual representation
// for each of the corresponding buttons.
type ButtonField struct {
	Common

	VariableText

	// ExportValues (optional) holds the "export value" corresponding to each
	// of the field's buttons, in order of the field's [Common.Widgets] slice.
	// If ExportValues is present, it must have the same length as Widgets.
	// The default is to use the names of the widgets' on-states instead.
	//
	// This corresponds to the /Opt entry in the PDF field dictionary.
	ExportValues []string

	// State (optional) is the on-state name of the button which is on, or
	// "Off" if no button is on.
	//
	// On write, an empty name can be used as a shorthand for "Off".
	//
	// This corresponds to the /V entry in the PDF field dictionary.
	State pdf.Name

	// DefaultState (optional) is the field's default state, used when the form
	// is reset.
	//
	// On write, an empty name can be used as a shorthand for "Off".
	//
	// This corresponds to the /DV entry in the PDF field dictionary.
	DefaultState pdf.Name
}

var _ Field = (*ButtonField)(nil)

// NewButtonField returns a new button field (field type "Btn") with the given
// partial name.
func NewButtonField(name string, tp ButtonVariant) *ButtonField {
	f := &ButtonField{}
	f.Name = name

	switch tp {
	case ButtonCheckbox:
		// pass
	case ButtonRadio:
		f.Flags = FieldRadio
	default:
		panic(fmt.Sprintf("invalid button variant %d", tp))
	}

	return f
}

// Variant reports whether the field is a check box or a group of radio
// buttons.
func (f *ButtonField) Variant() ButtonVariant {
	if f.Flags&FieldRadio != 0 {
		return ButtonRadio
	}
	return ButtonCheckbox
}

// FieldType implements the [Field] interface.
func (f *ButtonField) FieldType() pdf.Name { return "Btn" }

// ExportData returns the export value of the button which is on, inside a
// slice of length one.  If no button is on, nil is returned.
//
// This implements the [Field] interface.
func (f *ButtonField) ExportData() []string {
	if f.State == "" || f.State == "Off" {
		return nil
	}
	if i := slices.Index(f.OnStates(), f.State); i >= 0 && i < len(f.ExportValues) {
		return []string{f.ExportValues[i]}
	}
	return []string{string(f.State)}
}

func (f *ButtonField) fillDict(rm *pdf.ResourceManager, dict pdf.Dict) error {
	if err := f.VariableText.fillVarTextDict(rm, dict); err != nil {
		return err
	}
	if f.Flags&FieldPushbutton != 0 {
		return errors.New("button field with the Pushbutton flag")
	}
	if f.ExportValues != nil && len(f.ExportValues) != len(f.Widgets) {
		return errors.New("button field Opt length differs from widget count")
	}
	if len(f.ExportValues) > 0 {
		if err := pdf.CheckVersion(rm.Out, "button field Opt entry", pdf.V1_4); err != nil {
			return err
		}
		arr := make(pdf.Array, len(f.ExportValues))
		for i, s := range f.ExportValues {
			arr[i] = pdf.Text(s)
		}
		dict["Opt"] = arr
	}
	// An absent /V means that no button is on, and an absent /DV resets the
	// field to this.  The encoder never writes /V or /DV on a group, so a
	// field without them inherits no other value.
	if f.State != "Off" {
		fillName(dict, "V", f.State)
	}
	if f.DefaultState != "Off" {
		fillName(dict, "DV", f.DefaultState)
	}
	return f.fillRichText(rm, dict)
}

// fillValue adds the /V and /RV entries to dict, for [ValueEntries].  Since the
// dictionary may be patched into a field tree written by other software,
// where an ancestor can hold a /V, "Off" is written explicitly.
func (f *ButtonField) fillValue(rm *pdf.ResourceManager, dict pdf.Dict) error {
	v := f.State
	if v == "" {
		v = "Off"
	}
	dict["V"] = v
	return f.fillRichText(rm, dict)
}

// fillName writes a name entry, omitting it when the name is empty.
func fillName(dict pdf.Dict, key pdf.Name, n pdf.Name) {
	if n != "" {
		dict[key] = n
	}
}
