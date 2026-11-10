// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package action

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.6.2 12.6.4.17

// JavaScript represents a JavaScript action that executes ECMAScript.
type JavaScript struct {
	// JS is the JavaScript code to execute.
	JS *pdf.StringOrStream

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "JavaScript".
// This implements the [pdf.Action] interface.
func (a *JavaScript) ActionType() pdf.Name  { return TypeJavaScript }
func (a *JavaScript) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

func (a *JavaScript) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "JavaScript action", pdf.V1_3); err != nil {
		return nil, err
	}
	if a.JS == nil {
		return nil, errors.New("JavaScript action must have JS entry")
	}
	js, err := rm.Embed(*a.JS)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S":  pdf.Name(TypeJavaScript),
		"JS": js,
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

func decodeJavaScript(c pdf.Cursor, dict pdf.Dict) (*JavaScript, error) {
	jsObj, err := readStringOrStream(c, dict["JS"])
	if err != nil {
		return nil, err
	} else if jsObj == nil {
		return nil, pdf.Error("JavaScript action missing JS entry")
	}
	js, err := c.StringOrStream(jsObj)
	if err != nil {
		return nil, err
	}

	next, err := decodeNext(c, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &JavaScript{
		JS:   &js,
		Next: next,
	}, nil
}
