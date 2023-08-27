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

package tounicode

import (
	"bytes"
	"sort"

	"seehuhn.de/go/dag"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/type1"
)

// GetMapping returns the mapping information from info.
func (info *Info) GetMapping() map[charcode.CharCode][]rune {
	res := make(map[charcode.CharCode][]rune)
	for _, s := range info.Singles {
		res[s.Code] = s.Value
	}
	for _, r := range info.Ranges {
		if len(r.Values) == 1 {
			val := r.Values[0]
			for code := r.First; code <= r.Last; code++ {
				res[code] = val
				if code < r.Last {
					val = c(val)
					val[len(val)-1]++
				}
			}
		} else {
			for code := r.First; code <= r.Last; code++ {
				res[code] = r.Values[code-r.First]
			}
		}
	}
	return res
}

func c(rr []rune) []rune {
	res := make([]rune, len(rr))
	copy(res, rr)
	return res
}

// FromMapping constructs a ToUnicode cmap from the given mapping.
func FromMapping(CS charcode.CodeSpaceRange, m map[charcode.CharCode][]rune) *Info {
	info := &Info{
		ROS: &type1.CIDSystemInfo{
			Registry:   "Adobe",
			Ordering:   "UCS",
			Supplement: 0,
		},
		CS: CS,
	}
	info.SetMapping(m)
	info.makeName()
	return info
}

// SetMapping replaces the mapping information in info with the given mapping.
//
// To make efficient use of range entries, the generated mapping may be a
// superset of the original mapping, i.e. it may contain entries for charcodes
// which were not mapped in the original mapping.
func (info *Info) SetMapping(m map[charcode.CharCode][]rune) {
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
		} else if e < 0 {
			info.Ranges = append(info.Ranges, Range{
				First:  entries[v].code,
				Last:   entries[v-int(e)-1].code,
				Values: [][]rune{entries[v].value},
			})
		} else {
			var values [][]rune
			for i := v; i < v+int(e); i++ {
				values = append(values, entries[i].value)
			}
			info.Ranges = append(info.Ranges, Range{
				First:  entries[v].code,
				Last:   entries[v+int(e)-1].code,
				Values: values,
			})
		}
		v = g.To(v, e)
	}
}

type entry struct {
	code  charcode.CharCode
	value []rune
}

type encoder struct {
	cs   charcode.CodeSpaceRange
	mm   []entry
	bufR []rune
	buf0 pdf.String
	buf1 pdf.String
}

func (g *encoder) AppendEdges(ee []int16, v int) []int16 {
	if v < 0 || v >= len(g.mm) {
		return ee
	}

	m0 := g.mm[v]
	g.buf0 = g.cs.Append(g.buf0[:0], m0.code)
	g.bufR = append(g.bufR[:0], m0.value...)

	// Find the largest l such that entries v, ..., v+l-1 have codes which only
	// differ in the last byte, and such that the difference between values and
	// codes is constant.
	l := 1
vertexLoop:
	for v+l < len(g.mm) {
		m1 := g.mm[v+l]
		g.buf1 = g.cs.Append(g.buf1[:0], m1.code)
		if !bytes.Equal(g.buf0[:len(g.buf0)-1], g.buf1[:len(g.buf1)-1]) {
			break
		}
		if len(g.bufR) != len(m1.value) {
			break
		}
		for i := range g.bufR {
			if i < len(g.bufR)-1 {
				if g.bufR[i] != m1.value[i] {
					break vertexLoop
				}
			} else {
				if m1.code-m0.code != charcode.CharCode(m1.value[i]-g.bufR[i]) {
					break vertexLoop
				}
			}
		}
		l++
	}
	if l > 1 {
		// We can encode the entries v, ..., v+l-1 as a range of
		// l consecutive codes/values.  We use -l to indicate this
		// kind of range.
		ee = append(ee, int16(-l))
	} else {
		// We use 0 to indicate an entry in the Singles list
		ee = append(ee, 0)
	}

	// Find the largest k such that entries v, ..., v+k-1 have consecutive codes,
	// and the codes only differ in the last byte.
	k := 1
	for v+k < len(g.mm) && int(g.mm[v+k].code) == int(m0.code)+k {
		g.buf1 = g.cs.Append(g.buf1[:0], g.mm[v+k].code)
		if !bytes.Equal(g.buf0[:len(g.buf0)-1], g.buf1[:len(g.buf1)-1]) {
			break
		}
		k++
	}
	if k > l {
		// We can encode the entries v+l, ..., v+k-1 as a range
		// of k-l consecutive codes, using an array for the values.
		// We use +l to indicate this kind of range.
		ee = append(ee, int16(k))
	}

	return ee
}

func (g *encoder) Length(v int, e int16) uint32 {
	// For simplicity we ignore the cost of the "begin...end" operators.
	// For simplicity we assume all runes are in the BMP.

	cost := uint32(0)
	if e == 0 {
		g.buf0 = g.cs.Append(g.buf0[:0], g.mm[v].code)
		cost += 2*uint32(len(g.buf0)) + 3        // "<xx> "
		cost += 4*uint32(len(g.mm[v].value)) + 3 // "<xxxx>\n"
	} else if e < 0 {
		g.buf0 = g.cs.Append(g.buf0[:0], g.mm[v].code)
		g.buf1 = g.cs.Append(g.buf1[:0], g.mm[v-int(e)-1].code)
		cost += 2*uint32(len(g.buf0)+len(g.buf1)) + 6 // "<xx> <xx> "
		cost += 4*uint32(len(g.mm[v].value)) + 3      // "<xxxx>\n"
	} else if e > 0 {
		g.buf0 = g.cs.Append(g.buf0[:0], g.mm[v].code)
		g.buf1 = g.cs.Append(g.buf1[:0], g.mm[v+int(e)-1].code)
		cost += 2*uint32(len(g.buf0)+len(g.buf1)) + 8 // "<xx> <xx> []"
		for i := v; i < v+int(e); i++ {
			cost += 4*uint32(len(g.mm[v].value)) + 3 // "<xxxx> "
		}
	}

	return cost
}

func (g *encoder) To(v int, e int16) int {
	if e == 0 {
		return v + 1
	} else if e < 0 {
		return v - int(e)
	}
	return v + int(e)
}
