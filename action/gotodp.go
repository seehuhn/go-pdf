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

// PDF 2.0 sections: 12.6.2 12.6.4.5

package action

import (
	"seehuhn.de/go/pdf"
)

// GoToDp represents a go-to document part action that navigates to a
// specified DPart in the current document (PDF 2.0).
type GoToDp struct {
	// DPart (PDF 2.0) is a reference to the document part dictionary.
	DPart pdf.Reference

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "GoToDp".
// This implements the [Action] interface.
func (a *GoToDp) ActionType() Type { return TypeGoToDp }

func (a *GoToDp) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "GoToDp action", pdf.V2_0); err != nil {
		return nil, err
	}
	if a.DPart == 0 {
		return nil, pdf.Error("GoToDp action must have a DPart reference")
	}

	dict := pdf.Dict{
		"S":     pdf.Name(TypeGoToDp),
		"DPart": a.DPart,
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeGoToDp(x *pdf.Extractor, dict pdf.Dict) (*GoToDp, error) {
	ref, ok := dict["DPart"].(pdf.Reference)
	if !ok {
		return nil, pdf.Error("GoToDp action missing or invalid DPart entry")
	}

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &GoToDp{
		DPart: ref,
		Next:  next,
	}, nil
}
