// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
// This implements the [pdf.Action] interface.
func (a *Thread) ActionType() pdf.Name  { return TypeThread }
func (a *Thread) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

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

func decodeThread(c pdf.Cursor, dict pdf.Dict) (*Thread, error) {
	next, err := decodeNext(c, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &Thread{
		D:    dict["D"],
		B:    dict["B"],
		Next: next,
	}, nil
}
