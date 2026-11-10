// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package action

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.6.2 12.6.4.8

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
// This implements the [pdf.Action] interface.
func (a *URI) ActionType() pdf.Name  { return TypeURI }
func (a *URI) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

func (a *URI) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "URI action", pdf.V1_1); err != nil {
		return nil, err
	}
	if a.URI == "" {
		return nil, errors.New("URI action must have a non-empty URI")
	}

	dict := pdf.Dict{
		"S":   pdf.Name(TypeURI),
		"URI": pdf.String(a.URI),
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
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

func decodeURI(c pdf.Cursor, dict pdf.Dict) (*URI, error) {
	uri, err := c.String(dict["URI"])
	if err != nil {
		return nil, err
	}
	if len(uri) == 0 {
		return nil, pdf.Error("URI action: missing URI")
	}

	isMap, _, err := c.OptionalBoolean(dict["IsMap"])
	if err != nil {
		return nil, err
	}

	next, err := decodeNext(c, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &URI{
		URI:   string(uri),
		IsMap: bool(isMap),
		Next:  next,
	}, nil
}
