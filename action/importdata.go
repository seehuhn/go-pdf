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

package action

import (
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
// This implements the [Action] interface.
func (a *ImportData) ActionType() Type { return TypeImportData }

func (a *ImportData) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "ImportData action", pdf.V1_2); err != nil {
		return nil, err
	}
	if a.F == nil {
		return nil, pdf.Error("ImportData action must have F entry")
	}

	fn, err := rm.Embed(a.F)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeImportData),
		"F": fn,
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeImportData(x *pdf.Extractor, dict pdf.Dict) (*ImportData, error) {
	f, err := file.ExtractSpecification(x, dict["F"])
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, pdf.Error("ImportData action missing F entry")
	}

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &ImportData{
		F:    f,
		Next: next,
	}, nil
}
