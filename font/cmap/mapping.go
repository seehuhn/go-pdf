// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package cmap

import (
	"bytes"
	"sort"

	"seehuhn.de/go/dag"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/type1"
)

func (info *Info) GetMapping() map[charcode.CharCode]type1.CID {
	res := make(map[charcode.CharCode]type1.CID)
	for _, s := range info.Singles {
		res[s.Code] = s.Value
	}
	for _, r := range info.Ranges {
		val := r.Value
		for code := r.First; code <= r.Last; code++ {
			res[code] = val
			val++
		}
	}
	return res
}

// SetMapping overwrites the mapping information in info with the given
// mapping.
//
// To make efficient use of range entries, the generated mapping may be a
// superset of the original mapping, i.e. it may contain entries for charcodes
// which were not mapped in the original mapping.
func (info *Info) SetMapping(m map[charcode.CharCode]type1.CID) {
	entries := make([]entry, 0, len(m))
	for code, val := range m {
		entries = append(entries, entry{code, val})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].code < entries[j].code
	})

	g := &encoder{
		cs: info.CS,
		mm: entries,
	}
	ee, err := dag.ShortestPath[int16, uint32](g, len(entries))
	if err != nil {
		panic(err)
	}

	info.Singles = info.Singles[:0]
	info.Ranges = info.Ranges[:0]
	v := 0
	for _, e := range ee {
		if e == 0 {
			info.Singles = append(info.Singles, Single{
				Code:  entries[v].code,
				Value: entries[v].value,
			})
		} else {
			info.Ranges = append(info.Ranges, Range{
				First: entries[v].code,
				Last:  entries[v+int(e)-1].code,
				Value: entries[v].value,
			})
		}
		v = g.To(v, e)
	}
}

type entry struct {
	code  charcode.CharCode
	value type1.CID
}

type encoder struct {
	cs   charcode.CodeSpaceRange
	mm   []entry
	buf0 pdf.String
	buf1 pdf.String
}

func (g *encoder) AppendEdges(ee []int16, v int) []int16 {
	if v < 0 || v >= len(g.mm) {
		return ee
	}

	m0 := g.mm[v]
	g.buf0 = g.cs.Append(g.buf0[:0], m0.code)

	// Find the largest l such that entries v, ..., v+l-1 have codes which only
	// differ in the last byte, and such that the difference between values and
	// codes is constant.
	l := 1
	for v+l < len(g.mm) {
		m1 := g.mm[v+l]
		g.buf1 = g.cs.Append(g.buf1[:0], m1.code)
		if !bytes.Equal(g.buf0[:len(g.buf0)-1], g.buf1[:len(g.buf1)-1]) {
			break
		}
		if m1.code-m0.code != charcode.CharCode(m1.value-m0.value) {
			break
		}
		l++
	}
	if l > 1 {
		// We can encode the entries v, ..., v+l-1 as a range of
		// l consecutive codes/values.  We use l to indicate this
		// kind of range.
		ee = append(ee, int16(l))
	} else {
		// We use 0 to indicate an entry in the Singles list
		ee = append(ee, 0)
	}

	return ee
}

func (g *encoder) Length(v int, e int16) uint32 {
	// For simplicity we ignore the cost of the "begin...end" operators.

	cost := uint32(0)
	if e == 0 {
		g.buf0 = g.cs.Append(g.buf0[:0], g.mm[v].code)
		cost += 2*uint32(len(g.buf0)) + 3 // "<xx> "
		cost += ndigit(g.mm[v].value) + 1 // "xxx\n"
	} else {
		g.buf0 = g.cs.Append(g.buf0[:0], g.mm[v].code)
		cost += 4*uint32(len(g.buf0)) + 6 // "<xx> <xx> "
		cost += ndigit(g.mm[v].value) + 1 // "xxx\n"
	}

	return cost
}

func ndigit(cid type1.CID) uint32 {
	if cid < 10 {
		return 1
	} else if cid < 100 {
		return 2
	} else if cid < 1000 {
		return 3
	} else if cid < 10000 {
		return 4
	} else {
		return 5
	}
}

func (g *encoder) To(v int, e int16) int {
	if e == 0 {
		return v + 1
	}
	return v + int(e)
}
