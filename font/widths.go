// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package font

import (
	"errors"
	"sort"

	"seehuhn.de/go/dag"

	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf"
)

// WidthInfo contains the FirstChar, LastChar and Widths entries of
// a PDF font dictionary, as well as the MissingWidth entry of the
// FontDescriptor dictionary.
type WidthInfo struct {
	FirstChar    pdf.Integer
	LastChar     pdf.Integer
	Widths       pdf.Array
	MissingWidth pdf.Number
}

// EncodeWidthsSimple encodes the glyph width information for a simple PDF font.
// The slice ww must have length 256 and is indexed by character code.
// Widths values are given in PDF glyph space units.
func EncodeWidthsSimple(ww []float64) *WidthInfo {
	// find FirstChar and LastChar
	cand := make(map[float64]bool)
	cand[ww[0]] = true
	cand[ww[255]] = true
	bestGain := 0
	FirstChar := 0
	LastChar := 255
	var MissingWidth pdf.Number
	for w := range cand {
		b := 255
		for b > 0 && ww[b] == w {
			b--
		}
		a := 0
		for a < b && ww[a] == w {
			a++
		}
		gain := (255 - b + a) * 4
		if w != 0 {
			gain -= 15
		}
		if gain > bestGain {
			bestGain = gain
			FirstChar = a
			LastChar = b
			MissingWidth = pdf.Number(w)
		}
	}

	Widths := make(pdf.Array, LastChar-FirstChar+1)
	for i := range Widths {
		w := ww[FirstChar+i]
		Widths[i] = pdf.Number(w)
	}

	return &WidthInfo{
		FirstChar:    pdf.Integer(FirstChar),
		LastChar:     pdf.Integer(LastChar),
		Widths:       Widths,
		MissingWidth: MissingWidth,
	}
}

// EncodeWidthsComposite constructs the W and DW entries for a CIDFont dictionary.
func EncodeWidthsComposite(widths map[type1.CID]float64, v pdf.Version) (float64, pdf.Array) {
	var ww []cidWidth
	for cid, w := range widths {
		ww = append(ww, cidWidth{cid, w})
	}
	sort.Slice(ww, func(i, j int) bool {
		return ww[i].CID < ww[j].CID
	})

	// In pdf versions up to 1.7, "DW" is defined to be an integer.
	// From 2.0 onwards, it is defined to be a "number".

	// TODO(voss): be more clever here
	// TODO(voss): do we need to use the widths[0] here?
	dw := 1000.0
	if v >= pdf.V2_0 {
		dw = mostFrequent(ww)
	}

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

	return dw, res
}

type cidWidth struct {
	CID        type1.CID
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
	if ww[v].GlyphWidth == g.dw {
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

// DecodeWidthsComposite decodes the W and DW entries of a CIDFont dictionary.
func DecodeWidthsComposite(r pdf.Getter, ref pdf.Object, dw float64) (map[type1.CID]float64, error) {
	w, err := pdf.GetArray(r, ref)
	if err != nil {
		return nil, err
	}

	res := make(map[type1.CID]float64)
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
			if len(w) < 3 {
				break
			}
			wi, err := pdf.GetNumber(r, w[2])
			if err != nil {
				return nil, err
			}
			for c := c0; c <= c1; c++ {
				cid := type1.CID(c)
				if pdf.Integer(cid) != c {
					return nil, &pdf.MalformedFileError{
						Err: errors.New("invalid W entry in CIDFont dictionary"),
					}
				}
				if float64(wi) != dw {
					res[cid] = float64(wi)
				}
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
				cid := type1.CID(c0)
				if pdf.Integer(cid) != c0 {
					return nil, &pdf.MalformedFileError{
						Err: errors.New("invalid W entry in CIDFont dictionary"),
					}
				}
				if float64(wi) != dw {
					res[cid] = float64(wi)
				}
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
