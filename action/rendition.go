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

// PDF 2.0 sections: 12.6.2 12.6.4.14

package action

import (
	"seehuhn.de/go/pdf"
)

// Rendition represents a rendition action that controls multimedia playback.
type Rendition struct {
	// R is the rendition object.
	R pdf.Object

	// AN is the screen annotation for playback.
	AN pdf.Reference

	// OP is the operation to perform (0-4).
	OP *int

	// JS is the ECMAScript to execute.
	JS pdf.Object

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "Rendition".
// This implements the [Action] interface.
func (a *Rendition) ActionType() Type { return TypeRendition }

func (a *Rendition) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "Rendition action", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeRendition),
	}

	if a.R != nil {
		dict["R"] = a.R
	}

	if a.AN != 0 {
		dict["AN"] = a.AN
	}

	if a.OP != nil {
		dict["OP"] = pdf.Integer(*a.OP)
	}

	if a.JS != nil {
		dict["JS"] = a.JS
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeRendition(x *pdf.Extractor, dict pdf.Dict) (*Rendition, error) {
	an, _ := dict["AN"].(pdf.Reference)

	var op *int
	if opVal, err := pdf.Optional(pdf.GetInteger(x.R, dict["OP"])); err == nil {
		i := int(opVal)
		op = &i
	}

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &Rendition{
		R:    dict["R"],
		AN:   an,
		OP:   op,
		JS:   dict["JS"],
		Next: next,
	}, nil
}
