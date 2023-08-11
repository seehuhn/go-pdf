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
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/dag"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
)

func (info *Info) ToMapping() map[charcode.CharCode][]rune {
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

// FromMapping overwrites the mapping information in info with the given mapping.
func (info *Info) FromMapping(m map[charcode.CharCode][]rune) {
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
	buf1 pdf.String
	buf2 pdf.String
}

func (g *encoder) AppendEdges(ee []int16, v int) []int16 {
	if v < 0 || v >= len(g.mm) {
		return ee
	}

	// Find the largest k such that entries v, ..., v+k-1 have consecutive codes,
	// and the codes only differ in the last byte.
	k := 1
	g.buf1 = g.cs.Append(g.buf1[:0], g.mm[v].code)
	for v+k < len(g.mm) && int(g.mm[v+k].code) == int(g.mm[v].code)+k {
		g.buf2 = g.cs.Append(g.buf2[:0], g.mm[v+k].code)
		if !bytes.Equal(g.buf1[:len(g.buf1)-1], g.buf2[:len(g.buf2)-1]) {
			break
		}
		k++
	}

	needSingle := true

	if k > 1 {
		// Find the largest l <= k such that the entries v, ..., v+l-1 have
		// consecutive values.
		l := 1
		for l < k && isConsecutive(g.mm[v+l-1].value, g.mm[v+l].value) {
			l++
		}
		if l > 1 {
			// We can encode the entries v, ..., v+l-1 as a range of
			// l consecutive codes/values.  We use -l to indicate this
			// kind of range.
			ee = append(ee, int16(-l))
			needSingle = false
		}
		if k > l {
			// We can encode the entries v+l, ..., v+k-1 as a range
			// of k-l consecutive codes, using an array for the values.
			// We use +l to indicate this kind of range.
			ee = append(ee, int16(k))
		}
	}

	if needSingle {
		// We use 0 to indicate an entry in the Singles list
		ee = append(ee, 0)
	}

	return ee
}

func isConsecutive(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a)-1; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return b[len(b)-1] == a[len(a)-1]+1
}

func (g *encoder) Length(v int, e int16) uint32 {
	// For simplicity we ignore the cost of the "begin...end" operators.
	// For simplicity we assume all runes are in the BMP.

	cost := uint32(0)
	if e == 0 {
		g.buf1 = g.cs.Append(g.buf1[:0], g.mm[v].code)
		cost += 2*uint32(len(g.buf1)) + 3        // "<xx> "
		cost += 4*uint32(len(g.mm[v].value)) + 3 // "<xxxx>\n"
	} else if e < 0 {
		g.buf1 = g.cs.Append(g.buf1[:0], g.mm[v].code)
		g.buf2 = g.cs.Append(g.buf2[:0], g.mm[v-int(e)-1].code)
		cost += 2*uint32(len(g.buf1)+len(g.buf2)) + 6 // "<xx> <xx> "
		cost += 4*uint32(len(g.mm[v].value)) + 3      // "<xxxx>\n"
	} else if e > 0 {
		g.buf1 = g.cs.Append(g.buf1[:0], g.mm[v].code)
		g.buf2 = g.cs.Append(g.buf2[:0], g.mm[v+int(e)-1].code)
		cost += 2*uint32(len(g.buf1)+len(g.buf2)) + 8 // "<xx> <xx> []"
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

func makeName(m map[charcode.CharCode][]rune) pdf.Name {
	codes := maps.Keys(m)
	sort.Slice(codes, func(i, j int) bool {
		return codes[i] < codes[j]
	})
	h := sha256.New()
	for _, k := range codes {
		binary.Write(h, binary.BigEndian, uint32(k))
		h.Write([]byte{byte(len(m[k]))})
		binary.Write(h, binary.BigEndian, m[k])
	}
	sum := h.Sum(nil)
	return pdf.Name(fmt.Sprintf("Seehuhn-%x", sum[:8]))
}
