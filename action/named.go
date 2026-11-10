// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package action

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.6.2 12.6.4.12

// Named represents a named action that executes a predefined action.
// Standard names include NextPage, PrevPage, FirstPage, LastPage.
type Named struct {
	// N is the name of the action to perform.
	N pdf.Name

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "Named".
// This implements the [pdf.Action] interface.
func (a *Named) ActionType() pdf.Name  { return TypeNamed }
func (a *Named) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

func (a *Named) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "Named action", pdf.V1_2); err != nil {
		return nil, err
	}
	if a.N == "" {
		return nil, errors.New("Named action must have a non-empty name")
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeNamed),
		"N": a.N,
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

func decodeNamed(c pdf.Cursor, dict pdf.Dict) (*Named, error) {
	n, err := c.Name(dict["N"])
	if err != nil {
		return nil, err
	}
	if n == "" {
		return nil, pdf.Error("Named action: missing N field")
	}

	next, err := decodeNext(c, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &Named{
		N:    n,
		Next: next,
	}, nil
}
