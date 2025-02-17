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
	"sort"

	"seehuhn.de/go/dag"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/cmap"
)

// EncodeComposite constructs the W and DW entries for a CIDFont dictionary.
func EncodeComposite(widths map[cmap.CID]float64, dw float64) pdf.Array {
	var ww []cidWidth
	for cid, w := range widths {
		ww = append(ww, cidWidth{cid, w})
	}
	sort.Slice(ww, func(i, j int) bool {
		return ww[i].CID < ww[j].CID
	})

	g := wwGraph{ww, dw}
	ee, err := dag.ShortestPath(g, len(ww))
	if err != nil {
		panic(err)
	}

	var res pdf.Array
	pos := 0
	for _, e := range ee {
		switch {
		case e > 0:
			wiScaled := pdf.Number(ww[pos].GlyphWidth)
			res = append(res,
				pdf.Integer(ww[pos].CID),
				pdf.Integer(ww[pos+int(e)-1].CID),
				wiScaled)
		case e < 0:
			var wi pdf.Array
			for i := pos; i < pos+int(-e); i++ {
				wi = append(wi, pdf.Number(ww[i].GlyphWidth))
			}
			res = append(res,
				pdf.Integer(ww[pos].CID),
				wi)
		}
		pos = g.To(pos, e)
	}

	return res
}

type cidWidth struct {
	CID        cmap.CID
	GlyphWidth float64
}

type wwGraph struct {
	ww []cidWidth
	dw float64
}

// An Edge encodes how the next CID width is encoded.
// The edge values have the following meaning:
//
//	e=0: the width of the next CID is the default width, so no entry is needed
//	e>0: the next e CIDs have the same width, encode as a range
//	e<0: the next -e entries have consecutive CIDs, encode as an array
type wwEdge int16

func (g wwGraph) AppendEdges(ee []wwEdge, v int) []wwEdge {
	ww := g.ww
	if math.Abs(ww[v].GlyphWidth-g.dw) < 0.01 {
		return append(ee, 0)
	}

	n := len(ww)

	// positive edges: sequences of CIDS with the same width
	i := v + 1
	for i < n && ww[i].GlyphWidth == ww[v].GlyphWidth {
		i++
	}
	if i > v+1 {
		ee = append(ee, wwEdge(i-v))
	}

	// negative edges: sequences of consecutive CIDs
	i = v
	for i < n && int(ww[i].CID)-int(ww[v].CID) == i-v {
		i++
		ee = append(ee, wwEdge(v-i))
	}

	return ee
}

func (g wwGraph) Length(v int, e wwEdge) int {
	// for simplicity we assume that all integers in the output have 3 digits
	if e == 0 {
		return 0
	} else if e > 0 {
		// "%d %d %d\n"
		return 12
	} else {
		// "%d [%d ... %d]\n"
		return 6 + 4*int(-e)
	}
}

func (g wwGraph) To(v int, e wwEdge) int {
	if e == 0 {
		return v + 1
	}
	step := int(e)
	if step < 0 {
		step = -step
	}
	return v + step
}

func mostFrequent(ww []cidWidth) float64 {
	hist := make(map[float64]int)
	for _, wi := range ww {
		hist[wi.GlyphWidth]++
	}

	bestCount := 0
	bestVal := 0.0
	for wi, count := range hist {
		if count > bestCount || (count == bestCount && wi < bestVal) {
			bestCount = count
			bestVal = wi
		}
	}
	return bestVal
}

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

// DecodeComposite decodes the W entry of a CIDFont dictionary.
func ExtractComposite(r pdf.Getter, obj pdf.Object) (map[cmap.CID]float64, error) {
	w, err := pdf.GetArray(r, obj)
	if err != nil {
		return nil, err
	}

	res := make(map[cmap.CID]float64)
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
				return nil, &pdf.MalformedFileError{
					Err: errors.New("invalid W entry in CIDFont dictionary"),
				}
			}
			wi, err := pdf.GetNumber(r, w[2])
			if err != nil {
				return nil, err
			}
			for c := c0; c <= c1; c++ {
				cid := cmap.CID(c)
				if pdf.Integer(cid) != c {
					return nil, &pdf.MalformedFileError{
						Err: errors.New("invalid W entry in CIDFont dictionary"),
					}
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
				cid := cmap.CID(c0)
				if pdf.Integer(cid) != c0 {
					return nil, &pdf.MalformedFileError{
						Err: errors.New("invalid W entry in CIDFont dictionary"),
					}
				}
				res[cid] = float64(wi)
				c0++
			}
			w = w[2:]
		}
	}
	if len(w) != 0 {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("invalid W entry in CIDFont dictionary"),
		}
	}

	return res, nil
}
