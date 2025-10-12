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

// PDF 2.0 sections: 12.6.2 12.6.4.12

package action

import (
	"seehuhn.de/go/pdf"
)

// Named represents a named action that executes a predefined action.
// Standard names include NextPage, PrevPage, FirstPage, LastPage.
type Named struct {
	// N is the name of the action to perform.
	N pdf.Name

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

func (a *Named) ActionType() Type { return TypeNamed }

func (a *Named) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "Named action", pdf.V1_2); err != nil {
		return nil, err
	}
	if a.N == "" {
		return nil, pdf.Error("Named action must have a non-empty name")
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeNamed),
		"N": a.N,
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeNamed(x *pdf.Extractor, dict pdf.Dict) (*Named, error) {
	n, err := pdf.GetName(x.R, dict["N"])
	if err != nil {
		return nil, err
	}

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &Named{
		N:    n,
		Next: next,
	}, nil
}
