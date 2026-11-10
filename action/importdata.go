// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package action

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
)

// PDF 2.0 sections: 12.6.2 12.7.6.4

// ImportData represents an import-data action that imports form field values
// from a file.
type ImportData struct {
	// F is the FDF file from which to import data.
	F *file.Specification

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "ImportData".
// This implements the [pdf.Action] interface.
func (a *ImportData) ActionType() pdf.Name  { return TypeImportData }
func (a *ImportData) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

func (a *ImportData) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "ImportData action", pdf.V1_2); err != nil {
		return nil, err
	}
	if a.F == nil {
		return nil, errors.New("ImportData action must have F entry")
	}

	fn, err := rm.Embed(a.F)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeImportData),
		"F": fn,
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

func decodeImportData(c pdf.Cursor, dict pdf.Dict) (*ImportData, error) {
	f, err := c.Decode(dict["F"], file.ExtractSpecification)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, pdf.Error("ImportData action missing F entry")
	}

	next, err := decodeNext(c, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &ImportData{
		F:    f,
		Next: next,
	}, nil
}
