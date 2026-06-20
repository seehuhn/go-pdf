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
)

// PDF 2.0 sections: 12.6.2 12.6.4.2

// GoTo represents a go-to action that navigates to a destination in the
// current document.
type GoTo struct {
	// Dest is the destination to jump to.
	Dest destination.Destination

	// SD (PDF 2.0) is the structure destination to jump to.
	// When present it takes precedence over Dest; Dest remains the
	// page-based fallback for readers that do not support it.
	SD destination.Destination

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "GoTo".
// This implements the [pdf.Action] interface.
func (a *GoTo) ActionType() pdf.Name  { return TypeGoTo }
func (a *GoTo) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

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
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
	}

	if a.SD != nil {
		if err := pdf.CheckVersion(rm.Out, "GoTo action SD entry", pdf.V2_0); err != nil {
			return nil, err
		}
		sdObj, err := a.SD.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["SD"] = sdObj
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

func decodeGoTo(c pdf.Cursor, dict pdf.Dict) (pdf.Action, error) {
	dest, err := pdf.Decode(c, dict["D"], destination.Decode)
	if err != nil {
		return nil, err
	}

	var sd destination.Destination
	if dict["SD"] != nil {
		sd, err = pdf.DecodeOptional(c, dict["SD"], destination.Decode)
		if err != nil {
			return nil, err
		}
	}

	next, err := pdf.Decode(c, dict["Next"], DecodeActionList)
	if err != nil {
		return nil, err
	}

	return &GoTo{
		Dest: dest,
		SD:   sd,
		Next: next,
	}, nil
}
