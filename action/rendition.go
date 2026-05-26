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
	"seehuhn.de/go/pdf/media"
	"seehuhn.de/go/pdf/optional"
)

// PDF 2.0 sections: 12.6.2 12.6.4.14

// Rendition represents a rendition action that controls multimedia playback.
type Rendition struct {
	// R (optional) is the rendition to play.  It is required when OP is 0 or 4.
	R media.Rendition

	// AN is the screen annotation for playback.
	AN pdf.Reference

	// OP is the operation to perform when the action is triggered.
	// Required if JS is not present; otherwise optional.
	// Valid values are:
	//   - 0: Play the rendition R, associating it with annotation AN.
	//        If a rendition is already associated, stop it first.
	//   - 1: Stop any rendition associated with AN and remove the association.
	//   - 2: Pause any rendition associated with AN.
	//   - 3: Resume any paused rendition associated with AN.
	//   - 4: Play the rendition R if none is associated with AN,
	//        or resume if paused; otherwise do nothing.
	OP optional.UInt

	// JS is the ECMAScript to execute.
	JS pdf.Object

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "Rendition".
// This implements the [pdf.Action] interface.
func (a *Rendition) ActionType() pdf.Name  { return TypeRendition }
func (a *Rendition) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

func (a *Rendition) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "Rendition action", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeRendition),
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
	}

	if a.R != nil {
		r, err := rm.Embed(a.R)
		if err != nil {
			return nil, err
		}
		dict["R"] = r
	}

	if a.AN != 0 {
		dict["AN"] = a.AN
	}

	if op, ok := a.OP.Get(); ok {
		if op > 4 {
			return nil, pdf.Error("Rendition action OP must be 0-4")
		}
		if a.AN == 0 {
			return nil, pdf.Error("Rendition action requires AN when OP is present")
		}
		if (op == 0 || op == 4) && a.R == nil {
			return nil, pdf.Error("Rendition action requires R when OP is 0 or 4")
		}
		dict["OP"] = pdf.Integer(op)
	} else if a.JS == nil {
		return nil, pdf.Error("Rendition action requires OP or JS")
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

func decodeRendition(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) (*Rendition, error) {
	an, _ := dict["AN"].(pdf.Reference)

	r, err := pdf.ExtractorGetOptional(x, path, dict["R"], media.ExtractRendition)
	if err != nil {
		return nil, err
	}

	var op optional.UInt
	if dict["OP"] != nil {
		if opVal, err := x.GetInteger(path, dict["OP"]); err == nil && opVal >= 0 && opVal <= 4 {
			op.Set(uint(opVal))
		}
		// else: invalid OP value, treat as absent
	}
	// An OP value requires AN, and OP 0 and 4 additionally require R.  Drop any
	// OP that cannot be honoured, so the decoded action stays encodable.
	if v, ok := op.Get(); ok && (an == 0 || ((v == 0 || v == 4) && r == nil)) {
		op = optional.UInt{}
	}
	// OP is required when JS is absent, and OP needs AN.  Without AN the action
	// can only be carried by JS; if JS is missing too, it cannot be salvaged.
	if _, ok := op.Get(); !ok && dict["JS"] == nil {
		switch {
		case an == 0:
			return nil, pdf.Error("rendition action without AN, OP or JS")
		case r != nil:
			op.Set(0)
		default:
			op.Set(1)
		}
	}

	next, err := pdf.ExtractorGet(x, path, dict["Next"], DecodeActionList)
	if err != nil {
		return nil, err
	}

	return &Rendition{
		R:    r,
		AN:   an,
		OP:   op,
		JS:   dict["JS"],
		Next: next,
	}, nil
}
