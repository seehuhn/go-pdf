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
)

// PDF 2.0 sections: 12.6.2 12.6.4.7

// Thread represents a thread action that begins reading an article thread.
type Thread struct {
	// D is the thread dictionary or file specification with thread.
	D pdf.Object

	// B specifies a bead in the thread.
	B pdf.Object

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "Thread".
// This implements the [Action] interface.
func (a *Thread) ActionType() Type { return TypeThread }

func (a *Thread) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "Thread action", pdf.V1_1); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeThread),
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
	}

	if a.D != nil {
		dict["D"] = a.D
	}

	if a.B != nil {
		dict["B"] = a.B
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

func decodeThread(x *pdf.Extractor, dict pdf.Dict) (*Thread, error) {
	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &Thread{
		D:    dict["D"],
		B:    dict["B"],
		Next: next,
	}, nil
}
