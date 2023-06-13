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

package cid

import (
	"math"
	"sort"

	"seehuhn.de/go/dag"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/sfnt/funit"
	"seehuhn.de/go/sfnt/type1"
)

// WidthRec maps a character identifier (CID) to a glyph width in font units.
type WidthRec struct {
	CID        type1.CID
	GlyphWidth funit.Int16
}

// EncodeWidths constructs the W and DW entries for a CIDFont dictionary.
// This function modifies ww, sorting it by increasing CID.
func EncodeWidths(ww []WidthRec, unitsPerEm uint16) (pdf.Integer, pdf.Array) {
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
	dwScaled := pdf.Integer(math.Round(dw.AsFloat(q)))

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

	return dwScaled, res
}

type wwGraph struct {
	ww []WidthRec
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

func mostFrequent(ww []WidthRec) funit.Int16 {
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
