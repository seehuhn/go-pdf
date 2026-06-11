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

// FieldTx is a text input form field (field type "Tx").
type FieldTx struct {
	FieldCommon
	VariableText

	// V (optional) is the field's text value, a text string or a text stream.
	//
	// This corresponds to the /V entry.
	V pdf.Object

	// DV (optional) is the field's default text value, used when the form is
	// reset.
	//
	// This corresponds to the /DV entry.
	DV pdf.Object

	// MaxLen is the maximum length of the field's text in characters. A value
	// of zero indicates that no maximum is set. The entry is inheritable; a
	// field with the [FieldComb] flag set must have a MaxLen, either of its
	// own or inherited from an ancestor.
	//
	// This corresponds to the /MaxLen entry.
	MaxLen int
}

var _ Field = (*FieldTx)(nil)

// Encode implements [pdf.Encoder]; see [encodeField].
func (f *FieldTx) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	return encodeField(rm, f)
}

// FieldType implements the [Field] interface.
func (f *FieldTx) FieldType() pdf.Name { return "Tx" }

func (f *FieldTx) ownValue() pdf.Object        { return f.V }
func (f *FieldTx) ownDefaultValue() pdf.Object { return f.DV }

func (f *FieldTx) fillTypeDict(rm *pdf.ResourceManager, dict pdf.Dict) error {
	if err := f.VariableText.fillDict(rm, dict); err != nil {
		return err
	}
	if f.V != nil {
		dict["V"] = f.V
	}
	if f.DV != nil {
		dict["DV"] = f.DV
	}
	// the Comb flag requires a MaxLen (both are inheritable) and may not be
	// combined with the Multiline, Password or FileSelect flags
	if ff := ResolvedFf(f); ff&FieldComb != 0 {
		if ff&(FieldMultiline|FieldPassword|FieldFileSelect) != 0 {
			return errors.New("Comb flag conflicts with Multiline, Password or FileSelect")
		}
		if ResolvedMaxLen(f) <= 0 {
			return errors.New("text field with Comb flag requires MaxLen")
		}
	}
	if f.MaxLen > 0 {
		dict["MaxLen"] = pdf.Integer(f.MaxLen)
	}
	return nil
}
