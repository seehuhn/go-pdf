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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/postscript/cid"
)

// getSimpleWidths populates ww with glyph widths from a font dictionary.
// It sets default widths for all glyphs, then updates specific widths from
// the Widths array in fontDict.
//
// Returns true if widths were successfully read.
func getSimpleWidths(ww []float64, r pdf.Getter, fontDict pdf.Dict, defaultWidth float64) bool {
	for c := range ww {
		ww[c] = defaultWidth
	}

	firstChar, _ := pdf.GetInteger(r, fontDict["FirstChar"])
	widths, _ := pdf.GetArray(r, fontDict["Widths"])
	if widths == nil || len(widths) > 256 || firstChar < 0 || firstChar >= 256 {
		return false
	}

	for i, w := range widths {
		w, err := pdf.GetNumber(r, w)
		if err != nil {
			continue
		}
		if code := firstChar + pdf.Integer(i); code < 256 {
			ww[code] = float64(w)
		}
	}
	return true
}

// decodeCompositeWidths reads glyph width information from a composite font's W array.
// Returns a map from CID to width value.
func decodeCompositeWidths(r pdf.Getter, obj pdf.Object) (map[cid.CID]float64, error) {
	w, err := pdf.GetArray(r, obj)
	if w == nil {
		return nil, err
	}

	res := make(map[cid.CID]float64)
	for len(w) > 1 {
		c0, err := pdf.GetInteger(r, w[0])
		if err != nil {
			return nil, err
		}
		obj1, err := pdf.Resolve(r, w[1])
		if err != nil {
			return nil, err
		}
		if c1, ok := obj1.(pdf.Integer); ok {
			if len(w) < 3 || c0 < 0 || c1 < c0 || c1-c0 > 65536 {
				return nil, pdf.Error("invalid W entry in CIDFont dictionary")
			}
			wi, err := pdf.GetNumber(r, w[2])
			if err != nil {
				return nil, err
			}
			for c := c0; c <= c1; c++ {
				cid := cid.CID(c)
				if pdf.Integer(cid) != c {
					return nil, pdf.Error("invalid W entry in CIDFont dictionary")
				}
				res[cid] = float64(wi)
			}
			w = w[3:]
		} else {
			wi, err := pdf.GetArray(r, w[1])
			if err != nil {
				return nil, err
			}
			for _, wiObj := range wi {
				wi, err := pdf.GetNumber(r, wiObj)
				if err != nil {
					return nil, err
				}
				cid := cid.CID(c0)
				if pdf.Integer(cid) != c0 {
					return nil, pdf.Error("invalid W entry in CIDFont dictionary")
				}
				res[cid] = float64(wi)
				c0++
			}
			w = w[2:]
		}
	}
	if len(w) != 0 {
		return nil, pdf.Error("invalid W entry in CIDFont dictionary")
	}

	return res, nil
}

// decodeVDefault reads default vertical metrics from a PDF array.
func decodeVDefault(r pdf.Getter, obj pdf.Object) (dict.DefaultVMetrics, error) {
	a, err := pdf.GetArray(r, obj)
	if err != nil {
		return dict.DefaultVMetrics{}, err
	}

	res := dict.DefaultVMetrics{
		OffsY:  880,
		DeltaY: -1000,
	}

	if len(a) > 0 {
		offsY, err := pdf.GetNumber(r, a[0])
		if err != nil {
			return dict.DefaultVMetrics{}, err
		}
		res.OffsY = float64(offsY)
	}
	if len(a) > 1 {
		deltaY, err := pdf.GetNumber(r, a[1])
		if err != nil {
			return dict.DefaultVMetrics{}, err
		}
		res.DeltaY = float64(deltaY)
	}

	return res, nil
}

// decodeVMetrics reads vertical metrics information from a composite font's W2 array.
// Returns a map from CID to vertical metrics.
func decodeVMetrics(r pdf.Getter, obj pdf.Object) (map[cid.CID]dict.VMetrics, error) {
	a, err := pdf.GetArray(r, obj)
	if a == nil {
		return nil, err
	}

	res := make(map[cid.CID]dict.VMetrics)
	for len(a) > 0 {
		if len(a) < 2 {
			return nil, errInvalidVMetrics
		}

		// The first element is always a CID value
		x, err := pdf.GetInteger(r, a[0])
		if err != nil {
			return nil, err
		} else if x < 0 || x > 65535 {
			return nil, errInvalidVMetrics
		}
		cidVal := cid.CID(x)

		// The second element could be an array or a CID value
		elem2, err := pdf.Resolve(r, a[1])
		if err != nil {
			return nil, err
		}
		switch obj := elem2.(type) {
		case pdf.Array: // Individual format: cid [dy1 ox1 oy1 ...]
			if len(obj)%3 != 0 {
				return nil, errInvalidVMetrics
			}

			for i := 0; i < len(obj); i += 3 {
				dy, err := pdf.GetNumber(r, obj[i])
				if err != nil {
					return nil, err
				}
				offsX, err := pdf.GetNumber(r, obj[i+1])
				if err != nil {
					return nil, err
				}
				offsY, err := pdf.GetNumber(r, obj[i+2])
				if err != nil {
					return nil, err
				}

				res[cidVal] = dict.VMetrics{
					DeltaY: float64(dy),
					OffsX:  float64(offsX),
					OffsY:  float64(offsY),
				}
				cidVal++
			}

			a = a[2:]

		case pdf.Integer: // Range format: cid1 cid2 dy ox oy
			if len(a) < 5 {
				return nil, errInvalidVMetrics
			}

			x, err := pdf.GetInteger(r, elem2)
			if err != nil {
				return nil, err
			} else if x < 0 || x > 65535 {
				return nil, errInvalidVMetrics
			}
			cidEnd := cid.CID(x)

			dy, err := pdf.GetNumber(r, a[2])
			if err != nil {
				return nil, err
			}
			offsX, err := pdf.GetNumber(r, a[3])
			if err != nil {
				return nil, err
			}
			offsY, err := pdf.GetNumber(r, a[4])
			if err != nil {
				return nil, err
			}

			for c := int(cidVal); c <= int(cidEnd); c++ {
				res[cid.CID(c)] = dict.VMetrics{
					DeltaY: float64(dy),
					OffsX:  float64(offsX),
					OffsY:  float64(offsY),
				}
			}

			a = a[5:]

		default:
			return nil, errInvalidVMetrics
		}
	}

	return res, nil
}

var (
	errInvalidVMetrics = pdf.Error("invalid vertical metrics")
)
