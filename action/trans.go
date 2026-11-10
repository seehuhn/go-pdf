// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package action

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/page/transition"
)

// PDF 2.0 sections: 12.6.2 12.6.4.15

// Trans represents a transition action that updates the display using
// a transition dictionary.
type Trans struct {
	// Trans is the transition dictionary.
	Trans *transition.Transition

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "Trans".
// This implements the [pdf.Action] interface.
func (a *Trans) ActionType() pdf.Name  { return TypeTrans }
func (a *Trans) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

func (a *Trans) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "Trans action", pdf.V1_5); err != nil {
		return nil, err
	}
	if a.Trans == nil {
		return nil, errors.New("Trans action must have Trans entry")
	}

	transObj, err := rm.Embed(a.Trans)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S":     pdf.Name(TypeTrans),
		"Trans": transObj,
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeTrans(c pdf.Cursor, dict pdf.Dict) (*Trans, error) {
	trans, err := c.Decode(dict["Trans"], transition.Extract)
	if err != nil {
		return nil, err
	}

	next, err := decodeNext(c, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &Trans{
		Trans: trans,
		Next:  next,
	}, nil
}
