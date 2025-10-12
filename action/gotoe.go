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

// PDF 2.0 sections: 12.6.2 12.6.4.4

package action

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
)

// GoToE represents a go-to embedded action that navigates to a destination
// in an embedded file.
type GoToE struct {
	// F is the file specification for the embedded file.
	F *file.Specification

	// D is the destination to jump to.
	D pdf.Object

	// NewWindow indicates whether to open in a new window.
	NewWindow *bool

	// T specifies the target for the embedded file.
	T pdf.Object

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

func (a *GoToE) ActionType() Type { return TypeGoToE }

func (a *GoToE) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "GoToE action", pdf.V1_6); err != nil {
		return nil, err
	}
	if a.D == nil {
		return nil, pdf.Error("GoToE action must have D entry")
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeGoToE),
		"D": a.D,
	}

	if a.F != nil {
		fn, err := rm.Embed(a.F)
		if err != nil {
			return nil, err
		}
		dict["F"] = fn
	}

	if a.NewWindow != nil {
		dict["NewWindow"] = pdf.Boolean(*a.NewWindow)
	}

	if a.T != nil {
		dict["T"] = a.T
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeGoToE(x *pdf.Extractor, dict pdf.Dict) (*GoToE, error) {
	d := dict["D"]
	if d == nil {
		return nil, pdf.Error("GoToE action missing D entry")
	}

	f, err := file.ExtractSpecification(x, dict["F"])
	if err != nil {
		return nil, err
	}

	var newWindow *bool
	if dict["NewWindow"] != nil {
		nw, _ := pdf.Optional(pdf.GetBoolean(x.R, dict["NewWindow"]))
		b := bool(nw)
		newWindow = &b
	}

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &GoToE{
		F:         f,
		D:         d,
		NewWindow: newWindow,
		T:         dict["T"],
		Next:      next,
	}, nil
}
