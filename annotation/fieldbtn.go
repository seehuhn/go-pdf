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

// PDF 2.0 sections: 12.7.4.2.2 12.7.4.2.3 12.7.4.2.4

// ButtonVariant identifies the kind of a button form field ([FieldBtn]).
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

// FieldBtn is a button form field (field type "Btn").
//
// The three button kinds — check box, radio button, and push button — share one
// dictionary layout and differ only in their flags, so a single type represents
// all of them. Use [FieldBtn.Variant] to obtain the kind; it is derived from the
// field's effective (possibly inherited) flags, so a button whose variant flag
// is inherited from an ancestor is classified correctly.
type FieldBtn struct {
	FieldCommon
	VariableText

	// Opt (optional) holds the export value of each of the field's widget
	// annotations, in the same order as its [FieldCommon.Kids]. It allows
	// widgets that share an on-state name to be told apart. It does not apply to
	// push buttons.
	//
	// This corresponds to the /Opt entry.
	Opt []string

	// V (optional) is the field's value, the on-state name of the selected
	// appearance ("Off" for none) of a check box or radio button. A push button
	// retains no value, so V is empty for that variant. An empty value indicates
	// that the field has no value of its own.
	//
	// This corresponds to the /V entry.
	V pdf.Name

	// DV (optional) is the field's default value, used when the form is reset.
	//
	// This corresponds to the /DV entry.
	DV pdf.Name
}

var _ Field = (*FieldBtn)(nil)

// Variant reports whether the button is a check box, radio button, or push
// button, derived from the field's effective (possibly inherited) flags.
func (f *FieldBtn) Variant() ButtonVariant {
	ff := ResolvedFf(f)
	switch {
	case ff&FieldPushbutton != 0:
		return ButtonPush
	case ff&FieldRadio != 0:
		return ButtonRadio
	default:
		return ButtonCheckbox
	}
}

// Encode implements [pdf.Encoder]; see [encodeField].
func (f *FieldBtn) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	return encodeField(rm, f)
}

// FieldType implements the [Field] interface.
func (f *FieldBtn) FieldType() pdf.Name { return "Btn" }

func (f *FieldBtn) ownValue() pdf.Object        { return optionalName(f.V) }
func (f *FieldBtn) ownDefaultValue() pdf.Object { return optionalName(f.DV) }

func (f *FieldBtn) fillTypeDict(rm *pdf.ResourceManager, dict pdf.Dict) error {
	if err := f.VariableText.fillDict(rm, dict); err != nil {
		return err
	}
	fillExportValues(dict, f.Opt)
	fillName(dict, "V", f.V)
	fillName(dict, "DV", f.DV)
	return nil
}

func (f *FieldBtn) decodeType(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) error {
	if err := f.VariableText.decode(x, path, dict); err != nil {
		return err
	}
	if err := decodeExportValues(x, path, dict, &f.Opt); err != nil {
		return err
	}
	if v, err := pdf.Optional(x.GetName(path, dict["V"])); err != nil {
		return err
	} else {
		f.V = v
	}
	if dv, err := pdf.Optional(x.GetName(path, dict["DV"])); err != nil {
		return err
	} else {
		f.DV = dv
	}
	return nil
}

// optionalName converts a button value to a PDF object for inheritance, mapping
// an empty name to an absent value.
func optionalName(n pdf.Name) pdf.Object {
	if n == "" {
		return nil
	}
	return n
}

// fillName writes a name entry, omitting it when the name is empty.
func fillName(dict pdf.Dict, key pdf.Name, n pdf.Name) {
	if n != "" {
		dict[key] = n
	}
}

// fillExportValues writes a button field's Opt array of export values.
func fillExportValues(dict pdf.Dict, opt []string) {
	if len(opt) == 0 {
		return
	}
	arr := make(pdf.Array, len(opt))
	for i, s := range opt {
		arr[i] = pdf.TextString(s)
	}
	dict["Opt"] = arr
}

// decodeExportValues reads a button field's Opt array of export values into out.
func decodeExportValues(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict, out *[]string) error {
	arr, err := pdf.Optional(x.GetArray(path, dict["Opt"]))
	if err != nil {
		return err
	}
	if len(arr) == 0 {
		return nil
	}
	opt := make([]string, 0, len(arr))
	for _, el := range arr {
		s, err := pdf.Optional(pdf.GetTextString(x.R, el))
		if err != nil {
			return err
		}
		opt = append(opt, string(s))
	}
	*out = opt
	return nil
}
