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

package extract

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics/softclip"
)

// SoftMaskDict reads a soft-mask dictionary from a PDF file.
// Returns nil without error if obj is nil or /None.
// Note: For soft-mask images, use [SoftMaskImage] instead.
func SoftMaskDict(x *pdf.Extractor, obj pdf.Object) (*softclip.Mask, error) {
	if obj == nil {
		return nil, nil
	}

	// Check for /None name (explicit absence of soft mask)
	resolved, err := pdf.Resolve(x.R, obj)
	if err != nil {
		return nil, err
	}
	if resolved == pdf.Name("None") {
		return nil, nil
	}

	dict, err := x.GetDict(obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, nil
	}

	m := &softclip.Mask{}

	// S - subtype (required)
	sName, err := x.GetName(dict["S"])
	if err != nil {
		return nil, err
	}
	switch sName {
	case "Alpha":
		m.S = softclip.Alpha
	case "Luminosity":
		m.S = softclip.Luminosity
	default:
		return nil, &pdf.MalformedFileError{
			Err: errors.New("invalid soft mask subtype: " + string(sName)),
		}
	}

	// G - transparency group XObject (required)
	gObj := dict["G"]
	if gObj == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("soft mask: missing G entry"),
		}
	}
	g, err := Form(x, gObj)
	if err != nil {
		return nil, err
	}
	if g.Group == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("soft mask: G must be a transparency group XObject"),
		}
	}
	m.G = g

	// BC - backdrop color (optional)
	if bcObj := dict["BC"]; bcObj != nil {
		bcArray, err := x.GetArray(bcObj)
		if err != nil {
			return nil, err
		}
		m.BC = make([]float64, len(bcArray))
		for i, v := range bcArray {
			num, err := x.GetNumber(v)
			if err != nil {
				return nil, err
			}
			m.BC[i] = num
		}
	}

	// TR - transfer function (optional)
	if trObj := dict["TR"]; trObj != nil {
		// Check for /Identity name
		trResolved, err := pdf.Resolve(x.R, trObj)
		if err != nil {
			return nil, err
		}
		if trResolved != pdf.Name("Identity") {
			tr, err := function.Extract(x, trObj)
			if err != nil {
				return nil, err
			}
			m.TR = tr
		}
		// If /Identity, m.TR stays nil (which represents identity)
	}

	return m, nil
}
