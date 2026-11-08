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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/opaque"
)

// PDF 2.0 sections: 12.7.5.5

// SignatureField is a digital signature form field.
//
// The signature value is kept opaque.
//
// Use [seehuhn.de/go/pdf/annotation.AddWidget] to add a visual representation
// of the field to a page.
type SignatureField struct {
	Common

	// V (optional) is the field's signature dictionary. The library treats this
	// value as opaque.
	V *opaque.Object

	// DV (optional) is the field's default value. The library treats this value
	// as opaque.
	DV *opaque.Object

	// Lock (optional) specifies the form fields that are locked when this
	// signature field is signed.
	Lock *SigFieldLock

	// SV (optional) constrains the properties of a signature applied to this
	// field.
	SV *SigSeedValue
}

var _ Field = (*SignatureField)(nil)

// NewSignatureField returns a new signature field (field type "Sig") with the
// given partial name.
func NewSignatureField(name string) *SignatureField {
	f := &SignatureField{}
	f.Name = name
	return f
}

// FieldType implements the [Field] interface.
func (f *SignatureField) FieldType() pdf.Name { return "Sig" }

// ExportData always returns nil, since the value of a signature field is a
// signature dictionary, not a string.
//
// This implements the [Field] interface.
func (f *SignatureField) ExportData() []string {
	return nil
}

func (f *SignatureField) fillDict(rm *pdf.ResourceManager, dict pdf.Dict) error {
	if err := pdf.CheckVersion(rm.Out, "signature field", pdf.V1_3); err != nil {
		return err
	}
	if err := f.fillValue(rm, dict); err != nil {
		return err
	}
	if f.DV != nil {
		dv, err := rm.Embed(f.DV)
		if err != nil {
			return err
		}
		dict["DV"] = dv
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

// fillValue adds the /V entry to dict.
func (f *SignatureField) fillValue(rm *pdf.ResourceManager, dict pdf.Dict) error {
	if f.V == nil {
		return nil
	}
	v, err := rm.Embed(f.V)
	if err != nil {
		return err
	}
	dict["V"] = v
	return nil
}
