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

// PDF 2.0 sections: 12.6.2 12.6.4.15

package action

import (
	"seehuhn.de/go/pdf"
)

// Trans represents a transition action that updates the display using
// a transition dictionary.
type Trans struct {
	// Trans is the transition dictionary.
	Trans pdf.Dict

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

func (a *Trans) ActionType() Type { return TypeTrans }

func (a *Trans) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "Trans action", pdf.V1_5); err != nil {
		return nil, err
	}
	if a.Trans == nil {
		return nil, pdf.Error("Trans action must have Trans entry")
	}

	dict := pdf.Dict{
		"S":     pdf.Name(TypeTrans),
		"Trans": a.Trans,
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeTrans(x *pdf.Extractor, dict pdf.Dict) (*Trans, error) {
	trans, err := pdf.GetDict(x.R, dict["Trans"])
	if err != nil {
		return nil, err
	}

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &Trans{
		Trans: trans,
		Next:  next,
	}, nil
}
