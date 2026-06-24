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

// PDF 2.0 sections: 12.7.5.4

// ChoiceOption is a single selectable item of a [ChoiceField]. Export is the
// value used when the form is submitted; Display is the text shown to the user.
// When the two are equal, the option is stored as a single string.
type ChoiceOption struct {
	Export  string
	Display string
}

// ChoiceField is a choice form field: a list box, or a combo box when the
// [FieldCombo] flag is set (field type "Ch").
type ChoiceField struct {
	Common
	VariableText

	// Opt holds the field's selectable options, in display order.
	//
	// This corresponds to the /Opt entry in the PDF field dictionary.
	Opt []ChoiceOption

	// TopIndex is the index in Opt of the first option visible in a scrollable
	// list box.
	//
	// This corresponds to the /TI entry in the PDF field dictionary.
	TopIndex int

	// Selected (optional) holds the indices into Opt of the currently selected
	// options, in ascending order.
	//
	// This corresponds to the /I entry in the PDF field dictionary.
	Selected []int

	// V (optional) is the field's value: the display string of the selected
	// option, or an array of strings for a multi-select field.
	//
	// This corresponds to the /V entry in the PDF field dictionary.
	V pdf.Object

	// DV (optional) is the field's default value, used when the form is reset.
	//
	// This corresponds to the /DV entry in the PDF field dictionary.
	DV pdf.Object
}

var _ Field = (*ChoiceField)(nil)

// FieldType implements the [Field] interface.
func (f *ChoiceField) FieldType() pdf.Name { return "Ch" }

func (f *ChoiceField) fillDict(rm *pdf.ResourceManager, dict pdf.Dict) error {
	if err := f.VariableText.fillVarTextDict(rm, dict); err != nil {
		return err
	}
	if len(f.Opt) > 0 {
		arr := make(pdf.Array, len(f.Opt))
		for i, o := range f.Opt {
			if o.Export == o.Display {
				arr[i] = pdf.TextString(o.Display)
			} else {
				arr[i] = pdf.Array{pdf.TextString(o.Export), pdf.TextString(o.Display)}
			}
		}
		dict["Opt"] = arr
	}
	if f.TopIndex > 0 {
		dict["TI"] = pdf.Integer(f.TopIndex)
	}
	if len(f.Selected) > 0 {
		if err := pdf.CheckVersion(rm.Out, "choice field I entry", pdf.V1_4); err != nil {
			return err
		}
		arr := make(pdf.Array, len(f.Selected))
		for i, idx := range f.Selected {
			arr[i] = pdf.Integer(idx)
		}
		dict["I"] = arr
	}
	if f.V != nil {
		dict["V"] = f.V
	}
	if f.DV != nil {
		dict["DV"] = f.DV
	}
	return nil
}
