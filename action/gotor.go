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
	"seehuhn.de/go/pdf/destination"
	"seehuhn.de/go/pdf/file"
)

// PDF 2.0 sections: 12.6.2 12.6.4.3

// GoToR represents a remote go-to action that navigates to a destination
// in another PDF file.
type GoToR struct {
	// F is the file specification for the target document.
	F *file.Specification

	// D is the destination to jump to.
	// For explicit destinations, the page target is a page number (integer),
	// not a page reference.
	D destination.Destination

	// SD (PDF 2.0) is the structure destination to jump to.
	// If present, should take precedence over D.
	SD pdf.Array

	// NewWindow specifies how the target document should be displayed.
	NewWindow NewWindowMode

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "GoToR".
// This implements the [Action] interface.
func (a *GoToR) ActionType() Type { return TypeGoToR }

func (a *GoToR) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "GoToR action", pdf.V1_1); err != nil {
		return nil, err
	}
	if a.F == nil {
		return nil, pdf.Error("GoToR action must have F entry")
	}
	if a.D == nil {
		return nil, pdf.Error("GoToR action must have D entry")
	}

	fn, err := rm.Embed(a.F)
	if err != nil {
		return nil, err
	}

	destObj, err := a.D.Encode(rm)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeGoToR),
		"F": fn,
		"D": destObj,
	}

	if a.SD != nil {
		if err := pdf.CheckVersion(rm.Out, "GoToR action SD entry", pdf.V2_0); err != nil {
			return nil, err
		}
		dict["SD"] = a.SD
	}

	if a.NewWindow != NewWindowDefault {
		if err := pdf.CheckVersion(rm.Out, "GoToR action NewWindow entry", pdf.V1_2); err != nil {
			return nil, err
		}
		dict["NewWindow"] = pdf.Boolean(a.NewWindow == NewWindowNew)
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

func decodeGoToR(x *pdf.Extractor, dict pdf.Dict) (*GoToR, error) {
	f, err := file.ExtractSpecification(x, dict["F"])
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, pdf.Error("GoToR action missing F entry")
	}

	dest, err := destination.Decode(x, dict["D"])
	if err != nil {
		return nil, err
	}
	if dest == nil {
		return nil, pdf.Error("GoToR action missing D entry")
	}

	sd, _ := x.GetArray(dict["SD"])

	newWindow := NewWindowDefault
	if dict["NewWindow"] != nil {
		nw, _ := pdf.Optional(x.GetBoolean(dict["NewWindow"]))
		if nw {
			newWindow = NewWindowNew
		} else {
			newWindow = NewWindowReplace
		}
	}

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &GoToR{
		F:         f,
		D:         dest,
		SD:        sd,
		NewWindow: newWindow,
		Next:      next,
	}, nil
}
