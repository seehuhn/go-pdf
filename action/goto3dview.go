// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
// This implements the [pdf.Action] interface.
func (a *GoTo3DView) ActionType() pdf.Name  { return TypeGoTo3DView }
func (a *GoTo3DView) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

func (a *GoTo3DView) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "GoTo3DView action", pdf.V1_6); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeGoTo3DView),
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
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

func decodeGoTo3DView(c pdf.Cursor, dict pdf.Dict) (*GoTo3DView, error) {
	next, err := decodeNext(c, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &GoTo3DView{
		TA:   dict["TA"],
		V:    dict["V"],
		Next: next,
	}, nil
}
