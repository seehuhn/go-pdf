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

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.7.5.3

// TextField is a text input form field.
//
// Use [seehuhn.de/go/pdf/annotation.AddWidget] to add a visual representation
// of the field to a page.
type TextField struct {
	Common

	VariableText

	// V (optional) is the field's text value.
	//
	// This corresponds to the /V entry in the PDF field dictionary.
	V *pdf.StringOrStream

	// DV (optional) is the field's default text value, used when the form is
	// reset.
	//
	// This corresponds to the /DV entry in the PDF field dictionary.
	DV *pdf.StringOrStream

	// MaxLen is the maximum length of the field's text in characters. A value
	// of zero indicates that no maximum is set. A field with the [FieldComb]
	// flag set must have a MaxLen.
	//
	// This corresponds to the /MaxLen entry in the PDF field dictionary.
	MaxLen int
}

var _ Field = (*TextField)(nil)

// FieldType implements the [Field] interface.
func (f *TextField) FieldType() pdf.Name { return "Tx" }

func (f *TextField) fillDict(rm *pdf.ResourceManager, dict pdf.Dict) error {
	if err := f.VariableText.fillVarTextDict(rm, dict); err != nil {
		return err
	}
	if f.V != nil {
		obj, err := rm.Embed(*f.V)
		if err != nil {
			return err
		}
		dict["V"] = obj
	}
	if f.DV != nil {
		obj, err := rm.Embed(*f.DV)
		if err != nil {
			return err
		}
		dict["DV"] = obj
	}
	// the Comb flag requires a MaxLen and may not be combined with the
	// Multiline, Password or FileSelect flags
	if f.Flags&FieldComb != 0 {
		if f.Flags&(FieldMultiline|FieldPassword|FieldFileSelect) != 0 {
			return errors.New("Comb flag conflicts with Multiline, Password or FileSelect")
		}
		if f.MaxLen <= 0 {
			return errors.New("text field with Comb flag requires MaxLen")
		}
	}
	if f.MaxLen > 0 {
		dict["MaxLen"] = pdf.Integer(f.MaxLen)
	}
	return nil
}
