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

// PDF 2.0 sections: 12.6.2 12.6.4.11

package action

import (
	"seehuhn.de/go/pdf"
)

// Hide represents a hide action that shows or hides annotations.
type Hide struct {
	// T specifies the annotation(s) to hide or show.
	// Can be: text string (annotation name), dictionary (annotation),
	// or array of strings/dictionaries.
	T pdf.Object

	// H indicates whether to hide (true) or show (false) the annotations.
	// Default is true.
	H bool

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "Hide".
// This implements the [Action] interface.
func (a *Hide) ActionType() Type { return TypeHide }

func (a *Hide) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "Hide action", pdf.V1_2); err != nil {
		return nil, err
	}
	if a.T == nil {
		return nil, pdf.Error("Hide action must specify target annotations")
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeHide),
		"T": a.T,
	}

	// only write H if false (true is default)
	if !a.H {
		dict["H"] = pdf.Boolean(false)
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeHide(x *pdf.Extractor, dict pdf.Dict) (*Hide, error) {
	t := dict["T"]
	if t == nil {
		return nil, pdf.Error("Hide action missing T entry")
	}

	h := true // default value
	if dict["H"] != nil {
		hVal, _ := pdf.Optional(x.GetBoolean(dict["H"]))
		h = bool(hVal)
	}

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &Hide{
		T:    t,
		H:    h,
		Next: next,
	}, nil
}
