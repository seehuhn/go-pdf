// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package action

import (
	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.6.2 12.6.4.18

// RichMediaExecute represents a rich-media-execute action that sends a command
// to a rich media annotation's handler (PDF 2.0).
type RichMediaExecute struct {
	// TA (PDF 2.0) is the target rich media annotation.
	TA pdf.Reference

	// TI (PDF 2.0) is the target instance in the annotation.
	TI pdf.Object

	// C (PDF 2.0) is the command to execute.
	C pdf.Object

	// A (PDF 2.0) is the arguments for the command.
	A pdf.Object

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "RichMediaExecute".
// This implements the [pdf.Action] interface.
func (a *RichMediaExecute) ActionType() pdf.Name  { return TypeRichMediaExecute }
func (a *RichMediaExecute) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

func (a *RichMediaExecute) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "RichMediaExecute action", pdf.V2_0); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeRichMediaExecute),
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
	}

	if a.TA != 0 {
		dict["TA"] = a.TA
	}

	if a.TI != nil {
		dict["TI"] = a.TI
	}

	if a.C != nil {
		dict["C"] = a.C
	}

	if a.A != nil {
		dict["A"] = a.A
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeRichMediaExecute(c pdf.Cursor, dict pdf.Dict) (*RichMediaExecute, error) {
	ta, _ := dict["TA"].(pdf.Reference)

	next, err := decodeNext(c, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &RichMediaExecute{
		TA:   ta,
		TI:   dict["TI"],
		C:    dict["C"],
		A:    dict["A"],
		Next: next,
	}, nil
}
