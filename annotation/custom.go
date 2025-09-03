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

import (
	"errors"
	"maps"

	"seehuhn.de/go/pdf"
)

type Custom struct {
	Common

	// Type is the subtype of the annotation.
	Type pdf.Name

	// Data contains the raw data of the annotation.
	// This must not include the /Type and /Subtype fields.
	// Any fields corresponding the entries in Common are ignored.
	Data pdf.Dict
}

var _ Annotation = (*Custom)(nil)

// AnnotationType returns the subtype of the custom annotation.
// This implements the [Annotation] interface.
func (c *Custom) AnnotationType() pdf.Name {
	return c.Type
}

func decodeCustom(x *pdf.Extractor, dict pdf.Dict) (*Custom, error) {
	r := x.R
	subtype, err := pdf.GetName(r, dict["Subtype"])
	if err != nil {
		return nil, err
	} else if subtype == "" {
		return nil, pdf.Error("missing annotation subtype")
	}

	c := &Custom{
		Type: subtype,
		Data: dict,
	}

	// Extract common annotation fields
	if err := decodeCommon(x, &c.Common, dict); err != nil {
		return nil, err
	}

	// Remove the entries for c.Common from the Data dict.
	all := []pdf.Name{
		"Type",
		"Subtype",
		"Rect",
		"Contents",
		"P",
		"NM",
		"M",
		"F",
		"AP",
		"AS",
		"Border",
		"C",
		"StructParent",
		"OC",
		"AF",
		"ca",
		"CA",
		"BM",
		"Lang",
	}
	for _, key := range all {
		delete(c.Data, key)
	}

	return c, nil
}

func (c *Custom) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if c.Type == "" {
		return nil, errors.New("missing annotation subtype")
	}
	if _, present := c.Data["Type"]; present {
		return nil, errors.New("unexpected Type field")
	}
	if _, present := c.Data["Subtype"]; present {
		return nil, errors.New("unexpected Subtype field")
	}

	// Start with the raw data
	d := make(pdf.Dict)
	maps.Copy(d, c.Data)

	// Add/override common annotation fields
	d["Subtype"] = c.Type
	if err := c.Common.fillDict(rm, d, isMarkup(c)); err != nil {
		return nil, err
	}

	return d, nil
}
