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

package dict

import (
	"slices"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/cid"
)

// DefaultVMetricsDefault is the value of DefaultVMetrics which is used, if no
// value is specified in the PDF file.  Using this value for a "DefaultVMetrics"
// field in a composite font dictionary will slightly reduce PDF file size.
var DefaultVMetricsDefault = DefaultVMetrics{
	OffsY:  880,
	DeltaY: -1000,
}

type DefaultVMetrics struct {
	// OffsY is the vertical component of the glyph position vector, in PDF
	// glyph space units.
	//
	// The effect of this is that the glyph is moved down by OffsY, compared to
	// horizontal writing.
	//
	// There is no separate OffsX default value for vertical writing. Instead,
	// half of the horizontal glyph width is used as the default.
	OffsY float64

	// DeltaY is the vertical displacement of the following glyph, in PDF glyph
	// space units. (The horizontal displacement is always zero.)
	//
	// This is normally negative, so that writing goes from top to bottom.
	DeltaY float64
}

func decodeVDefault(r pdf.Getter, obj pdf.Object) (DefaultVMetrics, error) {
	a, err := pdf.GetArray(r, obj)
	if err != nil {
		return DefaultVMetrics{}, err
	}

	res := DefaultVMetrics{
		OffsY:  880,
		DeltaY: -1000,
	}

	if len(a) > 0 {
		offsY, err := pdf.GetNumber(r, a[0])
		if err != nil {
			return DefaultVMetrics{}, err
		}
		res.OffsY = float64(offsY)
	}
	if len(a) > 1 {
		deltaY, err := pdf.GetNumber(r, a[1])
		if err != nil {
			return DefaultVMetrics{}, err
		}
		res.DeltaY = float64(deltaY)
	}

	return res, nil
}

func encodeVDefault(metrics DefaultVMetrics) pdf.Array {
	if metrics.OffsY == 880 && metrics.DeltaY == -1000 {
		return nil
	}
	return pdf.Array{
		pdf.Number(metrics.OffsY),
		pdf.Number(metrics.DeltaY),
	}
}

// VMetrics represents the vertical metrics for an individual CID
type VMetrics struct {
	// OffsX is the horizontal component of the glyph position vector, in PDF
	// glyph space units.
	//
	// The effect of this is that the glyph is moved left by OffsX, compared to
	// horizontal writing.
	OffsX float64

	// OffsY is the vertical component of the glyph position vector, in PDF
	// glyph space units.
	//
	// The effect of this is that the glyph is moved down by OffsY, compared to
	// horizontal writing.
	OffsY float64

	// DeltaY is the vertical displacement for the following glyph.
	// (The horizontal displacement is always zero.)
	//
	// This is normally negative, so that writing goes from top to bottom.
	DeltaY float64
}

func decodeVMetrics(r pdf.Getter, obj pdf.Object) (map[cid.CID]VMetrics, error) {
	a, err := pdf.GetArray(r, obj)
	if a == nil {
		return nil, err
	}

	res := make(map[cid.CID]VMetrics)
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

				res[cidVal] = VMetrics{
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
				res[cid.CID(c)] = VMetrics{
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

func encodeVMetrics(metrics map[cid.CID]VMetrics) pdf.Array {
	if metrics == nil {
		return nil
	}

	res := pdf.Array{}

	cids := maps.Keys(metrics)
	slices.Sort(cids)
	for len(cids) > 0 {
		// Try to find a range of consecutive CIDs
		// with pairwise different metrics:
		end := 0
		for end < len(cids) {
			if end > 0 && cids[end-1]+1 != cids[end] {
				// not consecutive
				break
			}
			if end+1 < len(cids) &&
				cids[end]+1 == cids[end+1] &&
				metrics[cids[end]] == metrics[cids[end+1]] {
				// increasing enc would split consecutive CIDs with the same metrics
				break
			}
			end++
		}
		if end > 0 {
			// Individual format: cid [dy1 ox1 oy1 ...]
			data := make(pdf.Array, 0, 3*end)
			for i := 0; i < end; i++ {
				cid := cids[i]
				data = append(data,
					pdf.Number(metrics[cid].DeltaY),
					pdf.Number(metrics[cid].OffsX),
					pdf.Number(metrics[cid].OffsY))
			}
			res = append(res, pdf.Integer(cids[0]), data)
			cids = cids[end:]
		}

		// If we reach this point without having removed any elements from the
		// slice cids, the loop above must have terminated for end == 0.  This
		// means that all of the following are true:
		//  - end+1 < len(cids)
		//  - cids[end]+1 == cids[end+1]
		//  - metrics[cids[end]] == metrics[cids[end+1]]
		// Thus, in this case the following code will remove at least two cid
		// values.

		// Try to find a range of consecutive CIDs
		// with identical metrics:
		end = 1
		for end < len(cids) {
			if cids[end-1]+1 != cids[end] || metrics[cids[end-1]] != metrics[cids[end]] {
				// not consecutive
				break
			}
			end++
		}
		if end >= 2 {
			// Range format: cid1 cid2 dy ox oy
			res = append(res,
				pdf.Integer(cids[0]),
				pdf.Integer(cids[end-1]),
				pdf.Number(metrics[cids[0]].DeltaY),
				pdf.Number(metrics[cids[0]].OffsX),
				pdf.Number(metrics[cids[0]].OffsY))
			cids = cids[end:]
		}
	}

	return res
}

var (
	errInvalidVMetrics = pdf.Error("invalid vertical metrics")
)
