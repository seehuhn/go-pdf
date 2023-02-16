package tounicode

import (
	"sort"

	"golang.org/x/exp/slices"
	"seehuhn.de/go/dijkstra"
)

func (info *Info) ToMapping() []Single {
	var mappings []Single
	for _, s := range info.Singles {
		mappings = append(mappings, Single{
			Code:  s.Code,
			UTF16: s.UTF16,
		})
	}
	for _, r := range info.Ranges {
		if len(r.UTF16) == 1 {
			xx := r.UTF16[0]
			for code := r.First; code <= r.Last; code++ {
				mappings = append(mappings, Single{
					Code:  code,
					UTF16: slices.Clone(xx),
				})
				if canInc(xx) {
					xx = slices.Clone(xx)
					inc(xx)
				}
			}
		} else {
			for code := r.First; code <= r.Last; code++ {
				mappings = append(mappings, Single{
					Code:  code,
					UTF16: r.UTF16[code-r.First],
				})
			}
		}
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].Code < mappings[j].Code })
	return mappings
}

func FromMappings(mappings []Single) *Info {
	for i := 1; i < len(mappings); i++ {
		if mappings[i-1].Code >= mappings[i].Code {
			panic("mappings must be sorted by code")
		}
	}

	graph := optimizer(mappings)
	ee, err := dijkstra.ShortestPathSet[vertex, edge, int](graph, vertex{}, func(v vertex) bool {
		return v.pos == len(graph)
	})
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

type optimizer []Single

type edge struct {
	to   int
	repl [][]uint16
}

type vertex struct {
	pos                 int
	numSingle, numRange int
}

func (o optimizer) Edges(vv vertex) []edge {
	v := vv.pos

	thisRepl := o[v].UTF16

	res := make([]edge, 0, 3)

	// a bfchar
	res = append(res, edge{
		to: -1,
		repl: [][]uint16{
			thisRepl,
		},
	})

	// a bfrange with incrementing substitutions
	w1 := v + 1
	if len(thisRepl) == 1 {
		delta := int(thisRepl[0]) - int(o[v].Code)
		for w1 < len(o) && canInc(o[w1].UTF16) && int(o[w1].UTF16[0])-int(o[w1].Code) == delta && o[w1].Code%256 != 0 {
			w1++
		}
	}
	res = append(res, edge{
		to: w1 - 1,
		repl: [][]uint16{
			thisRepl,
		},
	})

	// a bfrange with an array of substitutions
	w2 := v + 1
	for w2 < len(o) && int(w2)-int(v) == int(o[w2].Code)-int(o[v].Code) && o[w2].Code%256 != 0 {
		w2++
	}
	if w2 > w1 {
		e := edge{
			to: w2 - 1,
		}
		for i := v; i < w2; i++ {
			e.repl = append(e.repl, o[i].UTF16)
		}
		res = append(res, e)
	}

	return res
}

func (o optimizer) Length(vv vertex, e edge) int {
	var length int
	if e.to < 0 { // a bfchar
		if vv.numSingle%100 == 0 {
			length += 22 // len("beginbfchar\nendbfchar\n")
		}

		length += 10 + 4*len(e.repl[0]) // len("<xxxx> <yyyy>\n")
	} else {
		if vv.numRange%100 == 0 {
			length += 24 // len("beginbfrange\nendbfrange\n")
		}

		if len(e.repl) == 1 { // a bfrange with incrementing substitutions
			length += 21 // len("<xxxx> <yyyy> <zzzz>\n")
		} else { // a bfrange with an array of substitutions
			length += 16 // len("<xxxx> <yyyy> []\n")-1
			for _, repl := range e.repl {
				length += 1 + 4*len(repl) // len(" <xxxx>")
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
