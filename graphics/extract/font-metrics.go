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
func getSimpleWidths(ww []float64, c pdf.Cursor, fontDict pdf.Dict, defaultWidth float64) bool {
	for i := range ww {
		ww[i] = defaultWidth
	}

	firstChar, _ := c.Integer(fontDict["FirstChar"])
	widths, _ := c.Array(fontDict["Widths"])
	if widths == nil || len(widths) > 256 || firstChar < 0 || firstChar >= 256 {
		return false
	}

	for i, w := range widths {
		w, err := c.Number(w)
		if err != nil {
			continue
		}
		if code := firstChar + pdf.Integer(i); code < 256 {
			ww[code] = w
		}
	}
	return true
}

// decodeCompositeWidths reads glyph width information from a composite font's W array.
// Returns a map from CID to width value.
func decodeCompositeWidths(c pdf.Cursor, obj pdf.Object) (map[cid.CID]float64, error) {
	w, err := c.Array(obj)
	if w == nil {
		return nil, err
	}

	res := make(map[cid.CID]float64)
	// CIDs are at most 65535, so a valid W array assigns at most 65536 widths.
	// Crafted overlapping or repeated ranges that exceed this are rejected to
	// bound both the map size and the loop work.
	count := 0
	for len(w) > 1 {
		c0, err := c.Integer(w[0])
		if err != nil {
			return nil, err
		}
		obj1, err := c.Resolve(w[1])
		if err != nil {
			return nil, err
		}
		if c1, ok := obj1.(pdf.Integer); ok {
			if len(w) < 3 || c0 < 0 || c1 < c0 || c1 > 65535 {
				return nil, pdf.Error("invalid W entry in CIDFont dictionary")
			}
			wi, err := c.Number(w[2])
			if err != nil {
				return nil, err
			}
			for c := c0; c <= c1; c++ {
				count++
				if count > 65536 {
					return nil, pdf.Error("invalid W entry in CIDFont dictionary")
				}
				res[cid.CID(c)] = wi
			}
			w = w[3:]
		} else {
			wi, err := c.Array(w[1])
			if err != nil {
				return nil, err
			}
			if c0 < 0 {
				return nil, pdf.Error("invalid W entry in CIDFont dictionary")
			}
			for _, wiObj := range wi {
				wi, err := c.Number(wiObj)
				if err != nil {
					return nil, err
				}
				if c0 > 65535 {
					return nil, pdf.Error("invalid W entry in CIDFont dictionary")
				}
				count++
				if count > 65536 {
					return nil, pdf.Error("invalid W entry in CIDFont dictionary")
				}
				res[cid.CID(c0)] = wi
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
func decodeVDefault(c pdf.Cursor, obj pdf.Object) (dict.DefaultVMetrics, error) {
	a, err := c.Array(obj)
	if err != nil {
		return dict.DefaultVMetrics{}, err
	}

	res := dict.DefaultVMetrics{
		OffsY:  880,
		DeltaY: -1000,
	}

	if len(a) > 0 {
		offsY, err := c.Number(a[0])
		if err != nil {
			return dict.DefaultVMetrics{}, err
		}
		res.OffsY = offsY
	}
	if len(a) > 1 {
		deltaY, err := c.Number(a[1])
		if err != nil {
			return dict.DefaultVMetrics{}, err
		}
		res.DeltaY = deltaY
	}

	return res, nil
}

// decodeVMetrics reads vertical metrics information from a composite font's W2 array.
// Returns a map from CID to vertical metrics.
func decodeVMetrics(c pdf.Cursor, obj pdf.Object) (map[cid.CID]dict.VMetrics, error) {
	a, err := c.Array(obj)
	if a == nil {
		return nil, err
	}

	res := make(map[cid.CID]dict.VMetrics)
	// CIDs are at most 65535, so a valid W2 array assigns at most 65536 entries.
	// Crafted overlapping or repeated ranges that exceed this are rejected to
	// bound both the map size and the loop work.
	count := 0
	for len(a) > 0 {
		if len(a) < 2 {
			return nil, errInvalidVMetrics
		}

		// The first element is always a CID value
		x, err := c.Integer(a[0])
		if err != nil {
			return nil, err
		} else if x < 0 || x > 65535 {
			return nil, errInvalidVMetrics
		}
		cidVal := cid.CID(x)

		// The second element could be an array or a CID value
		elem2, err := c.Resolve(a[1])
		if err != nil {
			return nil, err
		}
		switch obj := elem2.(type) {
		case pdf.Array: // Individual format: cid [dy1 ox1 oy1 ...]
			if len(obj)%3 != 0 {
				return nil, errInvalidVMetrics
			}

			for i := 0; i < len(obj); i += 3 {
				dy, err := c.Number(obj[i])
				if err != nil {
					return nil, err
				}
				offsX, err := c.Number(obj[i+1])
				if err != nil {
					return nil, err
				}
				offsY, err := c.Number(obj[i+2])
				if err != nil {
					return nil, err
				}

				if cidVal > 65535 {
					return nil, errInvalidVMetrics
				}
				count++
				if count > 65536 {
					return nil, errInvalidVMetrics
				}
				res[cidVal] = dict.VMetrics{
					DeltaY: dy,
					OffsX:  offsX,
					OffsY:  offsY,
				}
				cidVal++
			}

			a = a[2:]

		case pdf.Integer: // Range format: cid1 cid2 dy ox oy
			if len(a) < 5 {
				return nil, errInvalidVMetrics
			}

			x, err := c.Integer(elem2)
			if err != nil {
				return nil, err
			} else if x < 0 || x > 65535 {
				return nil, errInvalidVMetrics
			}
			cidEnd := cid.CID(x)

			dy, err := c.Number(a[2])
			if err != nil {
				return nil, err
			}
			offsX, err := c.Number(a[3])
			if err != nil {
				return nil, err
			}
			offsY, err := c.Number(a[4])
			if err != nil {
				return nil, err
			}

			for c := cidVal; c <= cidEnd; c++ {
				count++
				if count > 65536 {
					return nil, errInvalidVMetrics
				}
				res[c] = dict.VMetrics{
					DeltaY: dy,
					OffsX:  offsX,
					OffsY:  offsY,
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
