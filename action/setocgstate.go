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

// PDF 2.0 sections: 12.6.2 12.6.4.13

package action

import (
	"seehuhn.de/go/pdf"
)

// SetOCGState represents a set-OCG-state action that sets the state of
// optional content groups.
type SetOCGState struct {
	// State is an array of operations and OCG references.
	// Format: [name, ocg1, ocg2, ..., name, ocg3, ...]
	// where name is ON, OFF, or Toggle.
	State pdf.Array

	// PreserveRB indicates whether to preserve radio-button relationships.
	// Default is true.
	PreserveRB bool

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

func (a *SetOCGState) ActionType() Type { return TypeSetOCGState }

func (a *SetOCGState) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "SetOCGState action", pdf.V1_5); err != nil {
		return nil, err
	}
	if a.State == nil {
		return nil, pdf.Error("SetOCGState action must have State entry")
	}

	dict := pdf.Dict{
		"S":     pdf.Name(TypeSetOCGState),
		"State": a.State,
	}

	// only write PreserveRB if false (true is default)
	if !a.PreserveRB {
		dict["PreserveRB"] = pdf.Boolean(false)
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeSetOCGState(x *pdf.Extractor, dict pdf.Dict) (*SetOCGState, error) {
	state, err := pdf.GetArray(x.R, dict["State"])
	if err != nil {
		return nil, err
	}

	preserveRB := true // default value
	if dict["PreserveRB"] != nil {
		rb, _ := pdf.Optional(pdf.GetBoolean(x.R, dict["PreserveRB"]))
		preserveRB = bool(rb)
	}

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &SetOCGState{
		State:      state,
		PreserveRB: preserveRB,
		Next:       next,
	}, nil
}
