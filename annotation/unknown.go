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

package annotation

import "seehuhn.de/go/pdf"

type Unknown struct {
	Common

	// Type is the subtype of the annotation
	Type pdf.Name

	// Data contains the raw data of the annotation.
	Data pdf.Dict
}

var _ Annotation = (*Unknown)(nil)

// AnnotationType returns the subtype of the unknown annotation.
// This implements the [Annotation] interface.
func (u *Unknown) AnnotationType() pdf.Name {
	return u.Type
}

func (u *Unknown) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	// Start with the raw data
	dict := make(pdf.Dict)
	for k, v := range u.Data {
		dict[k] = v
	}

	// Ensure Type and Subtype are set
	if dict["Subtype"] == nil {
		dict["Subtype"] = pdf.Name("Unknown")
	}

	// Add/override common annotation fields
	if err := u.Common.fillDict(rm, dict, isMarkup(u)); err != nil {
		return nil, zero, err
	}

	return dict, zero, nil
}

func extractUnknown(r pdf.Getter, dict pdf.Dict, singleUse bool) (*Unknown, error) {
	subtype, err := pdf.GetName(r, dict["Subtype"])
	if err != nil {
		return nil, err
	} else if subtype == "" {
		return nil, pdf.Error("missing annotation subtype")
	}

	unknown := &Unknown{
		Type: subtype,
		Data: dict,
	}

	// Extract common annotation fields
	if err := extractCommon(r, &unknown.Common, dict, singleUse); err != nil {
		return nil, err
	}

	return unknown, nil
}
