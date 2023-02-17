package tounicode

import (
	"sort"

	"golang.org/x/exp/slices"

	"seehuhn.de/go/dag"

	"seehuhn.de/go/pdf/font/cmap"
)

func (info *Info) ToMapping() []Single {
	res := make(map[cmap.CID][]uint16)
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

func FromMappings(mappings []Single) *Info {
	for i := 1; i < len(mappings); i++ {
		if mappings[i-1].Code >= mappings[i].Code {
			panic("mappings must have strictly increasing codes")
		}
	}

	graph := optimizer(mappings)
	ee, err := dag.ShortestPathHist[edge, pathHistory, int](graph, len(mappings))
	if err != nil {
		panic(err)
	}

	info := &Info{
		CodeSpace: []CodeSpaceRange{{0x0000, 0xFFFF}},
	}
	v := 0
	for _, e := range ee {
		if e.to < 0 {
			info.Singles = append(info.Singles, Single{
				Code:  graph[v].Code,
				UTF16: e.repl[0],
			})
		} else {
			info.Ranges = append(info.Ranges, Range{
				First: graph[v].Code,
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

type pathHistory struct {
	numSingle, numRange int
}

func (o optimizer) AppendEdges(ee []edge, v int) []edge {
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

func (o optimizer) Length(v int, h pathHistory, e edge) int {
	var length int
	if e.to < 0 { // a bfchar
		if h.numSingle%100 == 0 {
			length += 22 // len("beginbfchar\nendbfchar\n")
		}

		length += 10 + 4*len(e.repl[0]) // len("<xxxx> <yyyy>\n")
	} else {
		if h.numRange%100 == 0 {
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

func (o optimizer) To(v int, e edge) int {
	if e.to < 0 {
		return v + 1
	} else {
		return e.to + 1
	}
}

func (o optimizer) UpdateHistory(h pathHistory, _ int, e edge) pathHistory {
	if e.to < 0 {
		h.numSingle++
	} else {
		h.numRange++
	}
	return h
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
