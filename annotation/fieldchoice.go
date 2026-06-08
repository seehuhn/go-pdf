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

import "seehuhn.de/go/pdf"

// PDF 2.0 sections: 12.7.4.4

// ChoiceOption is a single selectable item of a [FieldChoice]. Export is the
// value used when the form is submitted; Display is the text shown to the user.
// When the two are equal, the option is stored as a single string.
type ChoiceOption struct {
	Export  string
	Display string
}

// FieldChoice is a choice form field: a list box, or a combo box when the
// [FieldCombo] flag is set (field type "Ch").
type FieldChoice struct {
	FieldCommon
	VariableText

	// Opt holds the field's selectable options, in display order.
	//
	// This corresponds to the /Opt entry.
	Opt []ChoiceOption

	// TopIndex is the index in Opt of the first option visible in a scrollable
	// list box.
	//
	// This corresponds to the /TI entry.
	TopIndex int

	// Selected (optional) holds the indices into Opt of the currently selected
	// options, in ascending order.
	//
	// This corresponds to the /I entry.
	Selected []int

	// V (optional) is the field's value: the display string of the selected
	// option, or an array of strings for a multi-select field.
	//
	// This corresponds to the /V entry.
	V pdf.Object

	// DV (optional) is the field's default value, used when the form is reset.
	//
	// This corresponds to the /DV entry.
	DV pdf.Object
}

var _ Field = (*FieldChoice)(nil)

// Encode implements [pdf.Encoder]; see [encodeField].
func (f *FieldChoice) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	return encodeField(rm, f)
}

// FieldType implements the [Field] interface.
func (f *FieldChoice) FieldType() pdf.Name { return "Ch" }

func (f *FieldChoice) ownValue() pdf.Object        { return f.V }
func (f *FieldChoice) ownDefaultValue() pdf.Object { return f.DV }

func (f *FieldChoice) fillTypeDict(rm *pdf.ResourceManager, dict pdf.Dict) error {
	if err := f.VariableText.fillDict(rm, dict); err != nil {
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

func (f *FieldChoice) decodeType(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) error {
	if err := f.VariableText.decode(x, path, dict); err != nil {
		return err
	}

	if arr, err := pdf.Optional(x.GetArray(path, dict["Opt"])); err != nil {
		return err
	} else {
		for _, el := range arr {
			opt, ok := decodeChoiceOption(x, path, el)
			if ok {
				f.Opt = append(f.Opt, opt)
			}
		}
	}

	if ti, err := pdf.Optional(x.GetInteger(path, dict["TI"])); err != nil {
		return err
	} else if ti > 0 {
		f.TopIndex = int(ti)
	}

	if arr, err := pdf.Optional(x.GetArray(path, dict["I"])); err != nil {
		return err
	} else {
		for _, el := range arr {
			if idx, err := pdf.Optional(x.GetInteger(path, el)); err != nil {
				return err
			} else if idx >= 0 {
				f.Selected = append(f.Selected, int(idx))
			}
		}
	}

	f.V = dict["V"]
	f.DV = dict["DV"]
	return nil
}

// decodeChoiceOption reads a single /Opt entry, which is either a string (used
// for both export and display) or a two-element [export, display] array.
func decodeChoiceOption(x *pdf.Extractor, path *pdf.CycleCheck, el pdf.Object) (ChoiceOption, bool) {
	if arr, err := pdf.Optional(x.GetArray(path, el)); err == nil && len(arr) == 2 {
		export, err1 := pdf.Optional(pdf.GetTextString(x.R, arr[0]))
		display, err2 := pdf.Optional(pdf.GetTextString(x.R, arr[1]))
		if err1 == nil && err2 == nil {
			return ChoiceOption{Export: string(export), Display: string(display)}, true
		}
		return ChoiceOption{}, false
	}
	if s, err := pdf.Optional(pdf.GetTextString(x.R, el)); err == nil && el != nil {
		return ChoiceOption{Export: string(s), Display: string(s)}, true
	}
	return ChoiceOption{}, false
}
