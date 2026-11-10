// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package action

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/destination"
)

// PDF 2.0 sections: 12.6.2 12.6.4.2

// GoTo represents a go-to action that navigates to a destination in the
// current document.
type GoTo struct {
	// Dest is the destination to jump to.
	Dest destination.Destination

	// SD (PDF 2.0) is the structure destination to jump to.
	// When present it takes precedence over Dest; Dest remains the
	// page-based fallback for readers that do not support it.
	SD destination.Destination

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "GoTo".
// This implements the [pdf.Action] interface.
func (a *GoTo) ActionType() pdf.Name  { return TypeGoTo }
func (a *GoTo) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

func (a *GoTo) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "GoTo action", pdf.V1_1); err != nil {
		return nil, err
	}

	destObj, err := a.Dest.Encode(rm)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeGoTo),
		"D": destObj,
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
	}

	if a.SD != nil {
		if err := pdf.CheckVersion(rm.Out, "GoTo action SD entry", pdf.V2_0); err != nil {
			return nil, err
		}
		sdObj, err := a.SD.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["SD"] = sdObj
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

func decodeGoTo(c pdf.Cursor, dict pdf.Dict) (pdf.Action, error) {
	dest, err := c.Decode(dict["D"], destination.Decode)
	if err != nil {
		return nil, err
	}

	var sd destination.Destination
	if dict["SD"] != nil {
		sd, err = c.DecodeOptional(dict["SD"], destination.Decode)
		if err != nil {
			return nil, err
		}
	}

	next, err := decodeNext(c, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &GoTo{
		Dest: dest,
		SD:   sd,
		Next: next,
	}, nil
}
