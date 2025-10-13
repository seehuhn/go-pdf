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

// PDF 2.0 sections: 12.6.2 12.7.6.3

package action

import (
	"seehuhn.de/go/pdf"
)

// ResetForm represents a reset-form action that resets form fields to
// their default values.
type ResetForm struct {
	// Fields specifies which fields to reset.
	Fields pdf.Array

	// Flags specifies reset options.
	Flags int

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "ResetForm".
// This implements the [Action] interface.
func (a *ResetForm) ActionType() Type { return TypeResetForm }

func (a *ResetForm) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "ResetForm action", pdf.V1_2); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeResetForm),
	}

	if a.Fields != nil {
		dict["Fields"] = a.Fields
	}

	if a.Flags != 0 {
		dict["Flags"] = pdf.Integer(a.Flags)
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeResetForm(x *pdf.Extractor, dict pdf.Dict) (*ResetForm, error) {
	fields, _ := pdf.GetArray(x.R, dict["Fields"])
	flags, _ := pdf.Optional(pdf.GetInteger(x.R, dict["Flags"]))

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &ResetForm{
		Fields: fields,
		Flags:  int(flags),
		Next:   next,
	}, nil
}
