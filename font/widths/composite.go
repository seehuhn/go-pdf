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

package widths

import (
	"errors"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/cmap"
)

// DecodeComposite decodes the W and DW entries of a CIDFont dictionary.
func DecodeComposite(r pdf.Getter, ref pdf.Object, dwObj pdf.Object) (map[cmap.CID]float64, float64, error) {
	w, err := pdf.GetArray(r, ref)
	if err != nil {
		return nil, 0, err
	}

	dw, _ := pdf.GetNumber(r, dwObj)

	res := make(map[cmap.CID]float64)
	for len(w) > 1 {
		c0, err := pdf.GetInteger(r, w[0])
		if err != nil {
			return nil, 0, err
		}
		obj1, err := pdf.Resolve(r, w[1])
		if err != nil {
			return nil, 0, err
		}
		if c1, ok := obj1.(pdf.Integer); ok {
			if len(w) < 3 || c0 < 0 || c1 < c0 || c1-c0 > 65536 {
				return nil, 0, &pdf.MalformedFileError{
					Err: errors.New("invalid W entry in CIDFont dictionary"),
				}
			}
			wi, err := pdf.GetNumber(r, w[2])
			if err != nil {
				return nil, 0, err
			}
			for c := c0; c <= c1; c++ {
				cid := cmap.CID(c)
				if pdf.Integer(cid) != c {
					return nil, 0, &pdf.MalformedFileError{
						Err: errors.New("invalid W entry in CIDFont dictionary"),
					}
				}
				if math.Abs(float64(wi-dw)) > 1e-6 {
					res[cid] = float64(wi)
				}
			}
			w = w[3:]
		} else {
			wi, err := pdf.GetArray(r, w[1])
			if err != nil {
				return nil, 0, err
			}
			for _, wiObj := range wi {
				wi, err := pdf.GetNumber(r, wiObj)
				if err != nil {
					return nil, 0, err
				}
				cid := cmap.CID(c0)
				if pdf.Integer(cid) != c0 {
					return nil, 0, &pdf.MalformedFileError{
						Err: errors.New("invalid W entry in CIDFont dictionary"),
					}
				}
				if math.Abs(float64(wi-dw)) > 1e-6 {
					res[cid] = float64(wi)
				}
				c0++
			}
			w = w[2:]
		}
	}
	if len(w) != 0 {
		return nil, 0, &pdf.MalformedFileError{
			Err: errors.New("invalid W entry in CIDFont dictionary"),
		}
	}

	return res, float64(dw), nil
}
