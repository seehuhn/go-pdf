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
	"math"
	"sort"

	"seehuhn.de/go/dag"

	"seehuhn.de/go/postscript/funit"
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
func EncodeWidthsSimple(ww []funit.Int16, unitsPerEm uint16) *WidthInfo {
	q := 1000 / float64(unitsPerEm)

	// find FirstChar and LastChar
	cand := make(map[funit.Int16]bool)
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
			MissingWidth = pdf.Number(w.AsFloat(q))
		}
	}

	Widths := make(pdf.Array, LastChar-FirstChar+1)
	for i := range Widths {
		w := ww[FirstChar+i]
		Widths[i] = pdf.Number(w.AsFloat(q))
	}

	return &WidthInfo{
		FirstChar:    pdf.Integer(FirstChar),
		LastChar:     pdf.Integer(LastChar),
		Widths:       Widths,
		MissingWidth: MissingWidth,
	}
}

// CIDWidth maps a character identifier (CID) to a glyph width in font units.
type CIDWidth struct {
	CID        type1.CID
	GlyphWidth funit.Int16
}

// EncodeWidthsComposite constructs the W and DW entries for a CIDFont dictionary.
// This function modifies ww, sorting it by increasing CID.
//
// TODO(voss): use map[type1.CID]float64 instead of []CIDWidth?
func EncodeWidthsComposite(ww []CIDWidth, unitsPerEm uint16) (pdf.Number, pdf.Array) {
	sort.Slice(ww, func(i, j int) bool {
		return ww[i].CID < ww[j].CID
	})

	dw := mostFrequent(ww) // TODO(voss): be more clever here?

	g := wwGraph{ww, dw}
	ee, err := dag.ShortestPath[wwEdge, int](g, len(ww))
	if err != nil {
		panic(err)
	}

	q := 1000 / float64(unitsPerEm)
	dwScaled := dw.AsFloat(q)

	var res pdf.Array
	pos := 0
	for _, e := range ee {
		switch {
		case e > 0:
			wiScaled := pdf.Integer(math.Round(ww[pos].GlyphWidth.AsFloat(q)))
			res = append(res,
				pdf.Integer(ww[pos].CID),
				pdf.Integer(ww[pos+int(e)-1].CID),
				wiScaled)
		case e < 0:
			var wi pdf.Array
			for i := pos; i < pos+int(-e); i++ {
				wi = append(wi, pdf.Integer(math.Round(ww[i].GlyphWidth.AsFloat(q))))
			}
			res = append(res,
				pdf.Integer(ww[pos].CID),
				wi)
		}
		pos = g.To(pos, e)
	}

	return pdf.Number(dwScaled), res
}

type wwGraph struct {
	ww []CIDWidth
	dw funit.Int16
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

func mostFrequent(ww []CIDWidth) funit.Int16 {
	hist := make(map[funit.Int16]int)
	for _, wi := range ww {
		hist[wi.GlyphWidth]++
	}

	bestCount := 0
	bestVal := funit.Int16(0)
	for wi, count := range hist {
		if count > bestCount || (count == bestCount && wi < bestVal) {
			bestCount = count
			bestVal = wi
		}
	}
	return bestVal
}

// TODO(voss): make DecodeWidthsComposite and EncodeWidthsComposite
// inverse functions of each other.  Use this for testing/fuzzing.

// DecodeWidthsComposite decodes the W and DW entries of a CIDFont dictionary.
func DecodeWidthsComposite(r pdf.Getter, ref pdf.Object) (map[type1.CID]float64, float64, error) {
	dw, err := pdf.GetNumber(r, ref)
	if err != nil {
		return nil, 0, err
	}
	w, err := pdf.GetArray(r, ref)
	if err != nil {
		return nil, 0, err
	}

	res := make(map[type1.CID]float64)
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
			if len(w) < 3 {
				break
			}
			wi, err := pdf.GetNumber(r, w[2])
			if err != nil {
				return nil, 0, err
			}
			for c := c0; c <= c1; c++ {
				cid := type1.CID(c)
				if pdf.Integer(cid) != c {
					return nil, 0, &pdf.MalformedFileError{
						Err: errors.New("invalid W entry in CIDFont dictionary"),
					}
				}
				if wi != dw {
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
				cid := type1.CID(c0)
				if pdf.Integer(cid) != c0 {
					return nil, 0, &pdf.MalformedFileError{
						Err: errors.New("invalid W entry in CIDFont dictionary"),
					}
				}
				if wi != dw {
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
