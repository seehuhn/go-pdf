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

// PDF 2.0 sections: 12.7.4.5

// FieldSig is a digital signature form field (field type "Sig").
//
// The signature value is kept opaque.
type FieldSig struct {
	FieldCommon
	VariableText

	// V (optional) is the field's signature dictionary. The library treats this
	// value as opaque.
	//
	// This corresponds to the /V entry.
	V pdf.Object

	// DV (optional) is the field's default value. The library treats this value
	// as opaque.
	//
	// This corresponds to the /DV entry.
	DV pdf.Object

	// Lock (optional) specifies the form fields that are locked when this
	// signature field is signed.
	//
	// This corresponds to the /Lock entry.
	Lock *SigFieldLock

	// SV (optional) constrains the properties of a signature applied to this
	// field.
	//
	// This corresponds to the /SV entry.
	SV *SigSeedValue
}

var _ Field = (*FieldSig)(nil)

// Encode implements [pdf.Encoder]; see [encodeField].
func (f *FieldSig) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	return encodeField(rm, f)
}

// FieldType implements the [Field] interface.
func (f *FieldSig) FieldType() pdf.Name { return "Sig" }

func (f *FieldSig) ownValue() pdf.Object        { return f.V }
func (f *FieldSig) ownDefaultValue() pdf.Object { return f.DV }

func (f *FieldSig) fillTypeDict(rm *pdf.ResourceManager, dict pdf.Dict) error {
	if err := f.VariableText.fillDict(rm, dict); err != nil {
		return err
	}
	if f.V != nil {
		dict["V"] = f.V
	}
	if f.DV != nil {
		dict["DV"] = f.DV
	}
	if f.Lock != nil {
		lock, err := rm.Embed(f.Lock)
		if err != nil {
			return err
		}
		dict["Lock"] = lock
	}
	if f.SV != nil {
		sv, err := rm.Embed(f.SV)
		if err != nil {
			return err
		}
		dict["SV"] = sv
	}
	return nil
}

func (f *FieldSig) decodeType(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) error {
	if err := f.VariableText.decode(x, path, dict); err != nil {
		return err
	}
	f.V = dict["V"]
	f.DV = dict["DV"]
	if lock, err := pdf.ExtractorGetOptional(x, path, dict["Lock"], ExtractSigFieldLock); err != nil {
		return err
	} else {
		f.Lock = lock
	}
	if sv, err := pdf.ExtractorGetOptional(x, path, dict["SV"], ExtractSigSeedValue); err != nil {
		return err
	} else {
		f.SV = sv
	}
	return nil
}
