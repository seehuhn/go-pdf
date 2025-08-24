// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package file

import "seehuhn.de/go/pdf"

// PDF 2.0 sections: 7.6.7

// EncryptedPayload represents an encrypted payload dictionary.
// This dictionary provides details of the cryptographic filter needed
// to decrypt an encrypted payload document.
//
// This corresponds to an EncryptedPayload dictionary in PDF.
type EncryptedPayload struct {
	// FilterName is the name of the cryptographic filter used to encrypt the
	// encrypted payload document. This allows a PDF processor to easily
	// determine whether it has the appropriate cryptographic filter.
	//
	// This corresponds to the /Subtype entry in the PDF dictionary.
	FilterName pdf.Name

	// Version (optional) is the version number of the cryptographic filter
	// used to encrypt the encrypted payload referenced by this dictionary. The
	// value should consists of integers separated by period characters (e.g.,
	// "1.0", "2.1.1").
	Version string

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ pdf.Embedder[pdf.Unused] = (*EncryptedPayload)(nil)

// ExtractEncryptedPayload extracts an encrypted payload dictionary from a PDF object.
func ExtractEncryptedPayload(r pdf.Getter, obj pdf.Object) (*EncryptedPayload, error) {
	dict, err := pdf.GetDictTyped(r, obj, "EncryptedPayload")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Errorf("missing encrypted payload dictionary")
	}

	ep := &EncryptedPayload{}

	// Subtype (required)
	if subtype, err := pdf.GetName(r, dict["Subtype"]); err != nil {
		return nil, err
	} else if subtype == "" {
		return nil, pdf.Errorf("missing required Subtype in encrypted payload dictionary")
	} else {
		ep.FilterName = subtype
	}

	// Version (optional)
	if version, err := pdf.Optional(pdf.GetTextString(r, dict["Version"])); err != nil {
		return nil, err
	} else {
		ep.Version = string(version)
	}

	return ep, nil
}

// Embed converts the encrypted payload to a PDF object.
// This implements the pdf.Embedder interface.
func (ep *EncryptedPayload) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "encrypted payload dictionary", pdf.V2_0); err != nil {
		return nil, zero, err
	}

	if ep.FilterName == "" {
		return nil, zero, pdf.Errorf("missing required Subtype in encrypted payload dictionary")
	}

	dict := pdf.Dict{
		"Subtype": ep.FilterName,
	}

	// Optional Type field
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("EncryptedPayload")
	}

	// Version (optional)
	if ep.Version != "" {
		dict["Version"] = pdf.TextString(ep.Version)
	}

	if ep.SingleUse {
		return dict, zero, nil
	}

	ref := rm.Out.Alloc()
	err := rm.Out.Put(ref, dict)
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}
