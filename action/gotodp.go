// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package action

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.6.2 12.6.4.5

// GoToDp represents a go-to document part action that navigates to a
// specified DPart in the current document (PDF 2.0).
type GoToDp struct {
	// DPart (PDF 2.0) is a reference to the document part dictionary.
	DPart pdf.Reference

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "GoToDp".
// This implements the [pdf.Action] interface.
func (a *GoToDp) ActionType() pdf.Name  { return TypeGoToDp }
func (a *GoToDp) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

func (a *GoToDp) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "GoToDp action", pdf.V2_0); err != nil {
		return nil, err
	}
	if a.DPart == 0 {
		return nil, errors.New("GoToDp action must have a DPart reference")
	}

	dict := pdf.Dict{
		"S":  pdf.Name(TypeGoToDp),
		"Dp": a.DPart,
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

func decodeGoToDp(c pdf.Cursor, dict pdf.Dict) (*GoToDp, error) {
	ref, ok := dict["Dp"].(pdf.Reference)
	if !ok {
		return nil, pdf.Error("GoToDp action missing or invalid Dp entry")
	}

	next, err := decodeNext(c, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &GoToDp{
		DPart: ref,
		Next:  next,
	}, nil
}
