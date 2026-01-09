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

// PDF 2.0 sections: 12.6.2 12.6.4.16

// GoTo3DView represents a go-to-3D-view action that sets the current view
// of a 3D annotation.
type GoTo3DView struct {
	// TA is the target 3D annotation.
	TA pdf.Object

	// V is the 3D view to use.
	V pdf.Object

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "GoTo3DView".
// This implements the [Action] interface.
func (a *GoTo3DView) ActionType() Type { return TypeGoTo3DView }

func (a *GoTo3DView) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "GoTo3DView action", pdf.V1_6); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeGoTo3DView),
	}

	if a.TA != nil {
		dict["TA"] = a.TA
	}

	if a.V != nil {
		dict["V"] = a.V
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeGoTo3DView(x *pdf.Extractor, dict pdf.Dict) (*GoTo3DView, error) {
	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &GoTo3DView{
		TA:   dict["TA"],
		V:    dict["V"],
		Next: next,
	}, nil
}
