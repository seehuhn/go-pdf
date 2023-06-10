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

package main

import (
	"fmt"
	"math"
	"sort"

	"seehuhn.de/go/dag"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/sfnt/funit"
)

type widthRec struct {
	CID uint32
	W   funit.Int16
}

// This modifies ww.
func encodeWidths(ww []widthRec, q float64) (pdf.Integer, pdf.Array) {
	sort.Slice(ww, func(i, j int) bool {
		return ww[i].CID < ww[j].CID
	})

	dw := mostFrequent(ww)

	// TODO(voss): remove
	for _, w := range ww {
		fmt.Printf("%d %d\n", w.CID, pdf.Integer(math.Round(w.W.AsFloat(q))))
	}
	fmt.Printf("dw=%d\n", pdf.Integer(math.Round(dw.AsFloat(q))))

	g := graph{ww, dw}
	ee, err := dag.ShortestPath[edge, int](g, len(ww))
	if err != nil {
		panic(err)
	}

	var res pdf.Array
	pos := 0
	for _, e := range ee {
		switch {
		case e > 0:
			wi := pdf.Integer(math.Round(ww[pos].W.AsFloat(q)))
			res = append(res,
				pdf.Integer(ww[pos].CID),
				pdf.Integer(ww[pos+int(e)-1].CID),
				wi)
		case e < 0:
			var wi pdf.Array
			for i := pos; i < pos+int(-e); i++ {
				wi = append(wi, pdf.Integer(math.Round(ww[i].W.AsFloat(q))))
			}
			res = append(res,
				pdf.Integer(ww[pos].CID),
				wi)
		}
		pos = g.To(pos, e)
	}

	return pdf.Integer(math.Round(dw.AsFloat(q))), res
}

type graph struct {
	ww []widthRec
	dw funit.Int16
}
type edge int16

func (g graph) AppendEdges(ee []edge, v int) []edge {
	ww := g.ww
	if ww[v].W == g.dw {
		return append(ee, 0)
	}

	n := len(ww)

	// positive edges = sequences of CIDS with the same width
	i := v + 1
	for i < n && ww[i].W == ww[v].W {
		i++
	}
	ee = append(ee, edge(i-v))

	// negative edges = sequences of consecutive CIDs
	i = v + 1
	for i < n && int(ww[i].CID)-int(ww[v].CID) == i-v {
		i++
	}
	ee = append(ee, edge(v-i))

	return ee
}

func (g graph) Length(v int, e edge) int {
	// TODO(voss): be more accurate here?
	if e == 0 {
		return 0
	} else if e > 0 {
		return 12
	} else {
		return 6 + 4*int(-e)
	}
}

func (g graph) To(v int, e edge) int {
	if e == 0 {
		return v + 1
	}
	step := int(e)
	if step < 0 {
		step = -step
	}
	return v + step
}

func mostFrequent(ww []widthRec) funit.Int16 {
	hist := make(map[funit.Int16]int)
	for _, wi := range ww {
		hist[wi.W]++
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
