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

// PDF 2.0 sections: 12.6.2 12.6.4.17

package action

import (
	"seehuhn.de/go/pdf"
)

// JavaScript represents a JavaScript action that executes ECMAScript.
type JavaScript struct {
	// JS is the JavaScript code to execute.
	// Can be a text string or a stream.
	JS pdf.Object

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "JavaScript".
// This implements the [Action] interface.
func (a *JavaScript) ActionType() Type { return TypeJavaScript }

func (a *JavaScript) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "JavaScript action", pdf.V1_3); err != nil {
		return nil, err
	}
	if a.JS == nil {
		return nil, pdf.Error("JavaScript action must have JS entry")
	}

	dict := pdf.Dict{
		"S":  pdf.Name(TypeJavaScript),
		"JS": a.JS,
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeJavaScript(x *pdf.Extractor, dict pdf.Dict) (*JavaScript, error) {
	js := dict["JS"]
	if js == nil {
		return nil, pdf.Error("JavaScript action missing JS entry")
	}

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &JavaScript{
		JS:   js,
		Next: next,
	}, nil
}
