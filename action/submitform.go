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
	"errors"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.6.2 12.7.6.2

// SubmitForm represents a submit-form action that sends form data to a URL.
type SubmitForm struct {
	// F is the URL file specification giving the URL to which to submit the
	// form data.  This is a [pdf.String] or a [pdf.Dict] (or a [pdf.Reference]
	// to a file specification dictionary).
	F pdf.Object

	// Fields specifies which fields to submit.
	Fields pdf.Array

	// Flags specifies various submission options.
	Flags int

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "SubmitForm".
// This implements the [pdf.Action] interface.
func (a *SubmitForm) ActionType() pdf.Name  { return TypeSubmitForm }
func (a *SubmitForm) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

func (a *SubmitForm) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "SubmitForm action", pdf.V1_2); err != nil {
		return nil, err
	}
	if a.F == nil {
		return nil, errors.New("SubmitForm action must have F entry")
	}
	switch a.F.AsPDF(0).(type) {
	case pdf.String, pdf.Dict, pdf.Reference:
		// ok
	default:
		return nil, errors.New("invalid SubmitForm action F entry")
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeSubmitForm),
		"F": a.F,
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
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

func decodeSubmitForm(c pdf.Cursor, dict pdf.Dict) (*SubmitForm, error) {
	// F is a file specification: a string or a dictionary
	var f pdf.Object
	resolved, err := pdf.Optional(c.Resolve(dict["F"]))
	if err != nil {
		return nil, err
	}
	switch resolved.(type) {
	case pdf.String, pdf.Dict:
		f = resolved
	default:
		return nil, pdf.Error("SubmitForm action without valid F entry")
	}

	fields, err := pdf.Optional(c.Array(dict["Fields"]))
	if err != nil {
		return nil, err
	}
	flags, err := pdf.Optional(c.Integer(dict["Flags"]))
	if err != nil {
		return nil, err
	}

	next, err := decodeNext(c, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &SubmitForm{
		F:      f,
		Fields: fields,
		Flags:  int(flags),
		Next:   next,
	}, nil
}
