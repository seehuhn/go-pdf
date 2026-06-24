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
// [FieldCombo] flag is set.
//
// Use [seehuhn.de/go/pdf/annotation.AddWidget] to add a visual representation
// of the field to a page.
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

	// V (optional) holds the display strings of the currently selected options.
	// A single-selection field has at most one entry.
	//
	// This corresponds to the /V entry in the PDF field dictionary, which stores
	// a bare text string for a single selection and an array of text strings for
	// multiple selections.
	V []string

	// DV (optional) holds the default selection, used when the form is reset.
	//
	// This corresponds to the /DV entry in the PDF field dictionary.
	DV []string
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
	if v := choiceValueObject(f.V); v != nil {
		dict["V"] = v
	}
	if dv := choiceValueObject(f.DV); dv != nil {
		dict["DV"] = dv
	}
	return nil
}

// choiceValueObject converts a choice field's selection to its PDF
// representation: nil when nothing is selected, a bare text string for a single
// selection, and an array of text strings for multiple selections.
func choiceValueObject(vals []string) pdf.Object {
	switch len(vals) {
	case 0:
		return nil
	case 1:
		return pdf.TextString(vals[0])
	default:
		arr := make(pdf.Array, len(vals))
		for i, s := range vals {
			arr[i] = pdf.TextString(s)
		}
		return arr
	}
}
