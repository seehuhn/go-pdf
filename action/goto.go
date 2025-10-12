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

// PDF 2.0 sections: 12.6.2 12.6.4.2

package action

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/destination"
)

// GoTo represents a go-to action that navigates to a destination in the
// current document.
type GoTo struct {
	// Dest is the destination to jump to.
	Dest destination.Destination

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

func (a *GoTo) ActionType() Type { return TypeGoTo }

func (a *GoTo) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "GoTo action", pdf.V1_1); err != nil {
		return nil, err
	}

	destObj, err := a.Dest.Encode(rm)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeGoTo),
		"D": destObj,
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

func decodeGoTo(x *pdf.Extractor, dict pdf.Dict) (Action, error) {
	dest, err := destination.Decode(x, dict["D"])
	if err != nil {
		return nil, err
	}

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &GoTo{
		Dest: dest,
		Next: next,
	}, nil
}
