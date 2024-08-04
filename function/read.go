// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package function

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

// Read extracts a function from a PDF file.
func Read(r pdf.Getter, obj pdf.Object) (Func, error) {
	ref, isIndirect := obj.(pdf.Reference)
	singleUse := !isIndirect

	d, err := pdf.GetDict(r, obj)
	if err != nil {
		return nil, err
	}

	ft, ok := d["FunctionType"]
	if !ok {
		var loc []string
		if isIndirect {
			loc = []string{ref.String()}
		}
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /FunctionType entry"),
			Loc: loc,
		}
	}

	ftNum, err := pdf.GetInteger(r, ft)
	if err != nil {
		return nil, err
	}
	switch ftNum {
	case 2:
		y0, err := fromPDF(r, d["C0"])
		if err != nil {
			return nil, err
		}
		y1, err := fromPDF(r, d["C1"])
		if err != nil {
			return nil, err
		}
		gamma, err := pdf.GetNumber(r, d["N"])
		if err != nil {
			return nil, err
		}

		if len(y0) != len(y1) {
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("inconsistent length of /C0 and /C1 arrays"),
			}
		}

		res := &Type2{
			Y0:        y0,
			Y1:        y1,
			Gamma:     float64(gamma),
			SingleUse: singleUse,
		}
		return res, nil

	case 0, 3, 4:
		return nil, fmt.Errorf("function type %d not yet implemented", ftNum)
	default:
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("unsupported function type %d", ftNum),
		}
	}
}
