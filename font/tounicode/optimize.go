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
	"sort"

	"golang.org/x/exp/slices"
	"seehuhn.de/go/dag"

	"seehuhn.de/go/postscript/type1"
)

// ToMapping converts the ToUnicode CMap to a list of mappings.
// All ranges are expanded to single mappings.
// The returned mappings are sorted by the character code.
func (info *Info) ToMapping() []Single {
	res := make(map[type1.CID][]uint16)
	for _, s := range info.Singles {
		res[s.Code] = s.UTF16
	}
	for _, r := range info.Ranges {
		if len(r.UTF16) == 1 {
			xx := r.UTF16[0]
			for code := r.First; code <= r.Last; code++ {
				res[code] = xx
				if canInc(xx) {
					xx = slices.Clone(xx)
					inc(xx)
				}
			}
		} else {
			for code := r.First; code <= r.Last; code++ {
				res[code] = r.UTF16[code-r.First]
			}
		}
	}

	mappings := make([]Single, 0, len(res))
	for code, utf16 := range res {
		mappings = append(mappings, Single{code, utf16})
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].Code < mappings[j].Code })
	return mappings
}

// FromMappings creates a ToUnicode CMap from a list of mappings.
// The mappings must be sorted by code, and the code values must be
// strictly increasing.
// The returned CMap is optimized to minimize encoded CMap size.
func FromMappings(mappings []Single) *Info {
	for i := 1; i < len(mappings); i++ {
		if mappings[i-1].Code >= mappings[i].Code {
			panic("mappings must have strictly increasing codes")
		}
	}

	graph := optimizer(mappings)
	ee, err := dag.ShortestPathDyn[vertex, edge, int](graph, vertex{}, vertex{len(mappings), 0, 0})
	if err != nil {
		panic(err)
	}

	info := &Info{
		CodeSpace: []CodeSpaceRange{{0x0000, 0xFFFF}},
	}
	v := vertex{}
	for _, e := range ee {
		if e.to < 0 {
			info.Singles = append(info.Singles, Single{
				Code:  graph[v.pos].Code,
				UTF16: e.repl[0],
			})
		} else {
			info.Ranges = append(info.Ranges, Range{
				First: graph[v.pos].Code,
				Last:  graph[e.to].Code,
				UTF16: e.repl,
			})
		}
		v = graph.To(v, e)
	}
	return info
}

type edge struct {
	to   int
	repl [][]uint16
}

type vertex struct {
	pos                 int
	numSingle, numRange int
}

func (v vertex) Before(other vertex) bool {
	if v.pos != other.pos {
		return v.pos < other.pos
	}

	// The following would be accurate, but too expensive:
	//
	//     if v.numSingle != other.numSingle {
	//         return v.numSingle < other.numSingle
	//     }
	//     return v.numRange < other.numRange
	//
	// instead, we use the following heuristic:

	if v.numSingle == 0 && other.numSingle > 0 {
		return true
	} else if v.numSingle > 0 && other.numSingle == 0 {
		return false
	}
	return v.numRange == 0 && other.numRange > 0
}

type optimizer []Single

func (o optimizer) AppendEdges(ee []edge, vert vertex) []edge {
	v := vert.pos
	thisRepl := o[v].UTF16
	onlyThis := [][]uint16{thisRepl}

	// bfchar
	ee = append(ee, edge{
		to:   -1,
		repl: onlyThis,
	})

	// bfrange with incrementing substitutions
	w1 := v + 1
	if canInc(thisRepl) {
		repl := slices.Clone(thisRepl)
		for w1 < len(o) && o[w1].Code == o[w1-1].Code+1 && o[w1].Code%256 != 0 {
			inc(repl)
			if !slices.Equal(repl, o[w1].UTF16) {
				break
			}
			w1++
		}
	}
	ee = append(ee, edge{
		to:   w1 - 1,
		repl: onlyThis,
	})

	// bfrange with an array of substitutions
	w2 := v + 1
	for w2 < len(o) && o[w2].Code == o[w2-1].Code+1 && o[w2].Code%256 != 0 {
		w2++
	}
	if w2 > w1 {
		e := edge{
			to: w2 - 1,
		}
		e.repl = make([][]uint16, w2-v)
		for i := v; i < w2; i++ {
			e.repl[i-v] = o[i].UTF16
		}
		ee = append(ee, e)
	}

	return ee
}

func (o optimizer) Length(v vertex, e edge) int {
	var length int
	if e.to < 0 { // a bfchar
		if v.numSingle%100 == 0 {
			length += 22 // len("beginbfchar\nendbfchar\n")
		}

		length += 10 + 4*len(e.repl[0]) // len("<xxxx> <yyyy>\n")
	} else {
		if v.numRange%100 == 0 {
			length += 24 // len("beginbfrange\nendbfrange\n")
		}

		if len(e.repl) == 1 { // a bfrange with incrementing substitutions
			length += 17 + 4*len(e.repl[0]) // len("<xxxx> <yyyy> <zzzz>\n")
		} else { // a bfrange with an array of substitutions
			length += 16 // len("<xxxx> <yyyy> []\n") - 1
			for _, repl := range e.repl {
				length += 3 + 4*len(repl) // len(" <xxxx>")
			}
		}
	}
	return length
}

func (o optimizer) To(v vertex, e edge) vertex {
	if e.to < 0 {
		return vertex{
			pos:       v.pos + 1,
			numSingle: v.numSingle + 1,
			numRange:  v.numRange,
		}
	} else {
		return vertex{
			pos:       e.to + 1,
			numSingle: v.numSingle,
			numRange:  v.numRange + 1,
		}
	}
}

func canInc(xx []uint16) bool {
	if len(xx) == 0 {
		return false
	}
	if xx[0] >= 0xD800 && xx[0] <= 0xDBFF {
		return len(xx) == 2
	}
	return len(xx) == 1
}

func inc(xx []uint16) {
	isSurrogatePair := xx[0] >= 0xD800 && xx[0] <= 0xDBFF
	if isSurrogatePair {
		xx[1]++
		if xx[1] == 0xDC00 {
			xx[0]++
			xx[1] = 0xD800
		}
	} else {
		xx[0]++
	}
}
