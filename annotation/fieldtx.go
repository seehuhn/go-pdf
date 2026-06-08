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

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.7.4.3

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
	// of zero indicates that no maximum is set. It is required when the
	// [FieldComb] flag is set.
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
	// the Comb flag lays text out into MaxLen cells, so MaxLen is required when
	// it is set (the flag is inheritable; MaxLen is not)
	if ResolvedFf(f)&FieldComb != 0 && f.MaxLen <= 0 {
		return errors.New("text field with Comb flag requires MaxLen")
	}
	if f.MaxLen > 0 {
		dict["MaxLen"] = pdf.Integer(f.MaxLen)
	}
	return nil
}

func (f *FieldTx) decodeType(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) error {
	if err := f.VariableText.decode(x, path, dict); err != nil {
		return err
	}
	f.V = dict["V"]
	f.DV = dict["DV"]
	if ml, err := pdf.Optional(x.GetInteger(path, dict["MaxLen"])); err != nil {
		return err
	} else if ml > 0 {
		f.MaxLen = int(ml)
	}
	return nil
}
