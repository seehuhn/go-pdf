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
	"seehuhn.de/go/pdf/file"
)

// PDF 2.0 sections: 12.6.2 12.6.4.6

// Launch represents a launch action that launches an application or opens
// or prints a document.
type Launch struct {
	// F is the file specification for the file to launch or open.
	F *file.Specification

	// Win (deprecated in PDF 2.0) is Microsoft Windows launch parameters.
	Win pdf.Dict

	// Mac (deprecated in PDF 2.0) is Mac OS launch parameters.
	Mac pdf.Object

	// Unix (deprecated in PDF 2.0) is UNIX launch parameters.
	Unix pdf.Object

	// NewWindow specifies how the target document should be displayed.
	NewWindow NewWindowMode

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "Launch".
// This implements the [Action] interface.
func (a *Launch) ActionType() pdf.Name { return TypeLaunch }

func (a *Launch) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "Launch action", pdf.V1_1); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeLaunch),
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
	}

	if a.F != nil {
		fn, err := rm.Embed(a.F)
		if err != nil {
			return nil, err
		}
		dict["F"] = fn
	}

	if a.Win != nil {
		dict["Win"] = a.Win
	}

	if a.Mac != nil {
		dict["Mac"] = a.Mac
	}

	if a.Unix != nil {
		dict["Unix"] = a.Unix
	}

	if a.NewWindow != NewWindowDefault {
		if err := pdf.CheckVersion(rm.Out, "Launch action NewWindow entry", pdf.V1_2); err != nil {
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

func decodeLaunch(x *pdf.Extractor, dict pdf.Dict) (*Launch, error) {
	f, err := file.ExtractSpecification(x, dict["F"])
	if err != nil {
		return nil, err
	}

	win, _ := x.GetDict(dict["Win"])

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

	return &Launch{
		F:         f,
		Win:       win,
		Mac:       dict["Mac"],
		Unix:      dict["Unix"],
		NewWindow: newWindow,
		Next:      next,
	}, nil
}
