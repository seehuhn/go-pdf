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

// PDF 2.0 sections: 12.6.2 12.6.4.8

package action

import (
	"seehuhn.de/go/pdf"
)

// URI represents a URI action that resolves a uniform resource identifier.
type URI struct {
	// URI is the uniform resource identifier to resolve.
	URI string

	// IsMap indicates whether the URI is a map area.
	IsMap bool

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "URI".
// This implements the [Action] interface.
func (a *URI) ActionType() Type { return TypeURI }

func (a *URI) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "URI action", pdf.V1_1); err != nil {
		return nil, err
	}
	if a.URI == "" {
		return nil, pdf.Error("URI action must have a non-empty URI")
	}

	dict := pdf.Dict{
		"S":   pdf.Name(TypeURI),
		"URI": pdf.String(a.URI),
	}

	if a.IsMap {
		if err := pdf.CheckVersion(rm.Out, "URI action IsMap entry", pdf.V1_2); err != nil {
			return nil, err
		}
		dict["IsMap"] = pdf.Boolean(true)
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		if err := pdf.CheckVersion(rm.Out, "action Next entry", pdf.V1_2); err != nil {
			return nil, err
		}
		dict["Next"] = next
	}

	return dict, nil
}

func decodeURI(x *pdf.Extractor, dict pdf.Dict) (*URI, error) {
	uri, err := x.GetString(dict["URI"])
	if err != nil {
		return nil, err
	}

	isMap, _ := pdf.Optional(x.GetBoolean(dict["IsMap"]))

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &URI{
		URI:   string(uri),
		IsMap: bool(isMap),
		Next:  next,
	}, nil
}
