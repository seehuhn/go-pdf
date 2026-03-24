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

package action

import (
	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.6.2 12.6.4.13

// SetOCGState represents a set-OCG-state action that sets the state of
// optional content groups.
type SetOCGState struct {
	// State is an array of operations and OCG references.
	// Format: [name, ocg1, ocg2, ..., name, ocg3, ...]
	// where name is ON, OFF, or Toggle.
	State pdf.Array

	// IgnoreRBGroups, when true, causes radio-button state relationships
	// between optional content groups to be ignored.
	//
	// This corresponds to the PreserveRB entry in the PDF specification, but
	// with inverted meaning.
	IgnoreRBGroups bool

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "SetOCGState".
// This implements the [pdf.Action] interface.
func (a *SetOCGState) ActionType() pdf.Name  { return TypeSetOCGState }
func (a *SetOCGState) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

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
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
	}

	// only write PreserveRB when false (true is the PDF default)
	if a.IgnoreRBGroups {
		dict["PreserveRB"] = pdf.Boolean(false)
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeSetOCGState(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) (*SetOCGState, error) {
	state, err := x.GetArray(path, dict["State"])
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = pdf.Array{} // empty state = no-op action
	}

	ignoreRB := false // default: preserve radio-button groups
	if dict["PreserveRB"] != nil {
		rb, _ := pdf.Optional(x.GetBoolean(path, dict["PreserveRB"]))
		ignoreRB = !bool(rb)
	}

	next, err := DecodeActionList(x, path, dict["Next"], false)
	if err != nil {
		return nil, err
	}

	return &SetOCGState{
		State:          state,
		IgnoreRBGroups: ignoreRB,
		Next:           next,
	}, nil
}
