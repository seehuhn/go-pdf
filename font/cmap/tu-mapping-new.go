// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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
	"slices"
	"sort"
	"unicode/utf16"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/dag"
	"seehuhn.de/go/pdf/font/charcode"
)

// NewToUnicodeFile creates a ToUnicodeFile object.
func NewToUnicodeFile(codec *charcode.Codec, data map[charcode.Code]string) *ToUnicodeFile {
	res := &ToUnicodeFile{
		CodeSpaceRange: codec.CodeSpaceRange(),
	}

	// group together codes which only differ in the last byte
	type entry struct {
		code charcode.Code
		x    byte
	}
	ranges := make(map[string][]entry)
	var buf []byte
	for code := range data {
		buf = codec.AppendCode(buf[:0], code)
		l := len(buf)
		key := string(buf[:l-1])
		ranges[key] = append(ranges[key], entry{code, buf[l-1]})
	}

	// find all ranges, in sorted order
	keys := maps.Keys(ranges)
	sort.Slice(keys, func(i, j int) bool {
		return slices.Compare([]byte(keys[i]), []byte(keys[j])) < 0
	})

	// for each range, add the required CIDRanges and CIDSingles
	for _, key := range keys {
		info := ranges[key]
		sort.Slice(info, func(i, j int) bool {
			return info[i].x < info[j].x
		})

		start := 0
		for i := 1; i <= len(info); i++ {
			if i == len(info) || info[i].x != info[i-1].x+1 {
				first := make([]byte, len(key)+1)
				copy(first, key)
				first[len(key)] = info[start].x
				if i-start > 1 {
					last := make([]byte, len(key)+1)
					copy(last, key)
					last[len(key)] = info[i-1].x

					needsList := false
					for j := start; j < i-1; j++ {
						if data[info[j+1].code] != nextString(data[info[j].code]) {
							needsList = true
							break
						}
					}

					var values []string
					if needsList {
						values = make([]string, i-start)
						for j := start; j < i; j++ {
							values[j-start] = data[info[j].code]
						}
					} else {
						values = []string{data[info[start].code]}
					}

					res.Ranges = append(res.Ranges, ToUnicodeRange{
						First:  first,
						Last:   last,
						Values: values,
					})
				} else {
					res.Singles = append(res.Singles, ToUnicodeSingle{
						Code:  first,
						Value: data[info[start].code],
					})
				}
				start = i
			}
		}
	}

	return res
}

func nextString(s string) string {
	rr := []rune(s)
	if len(rr) == 0 {
		return ""
	}
	rr[len(rr)-1]++
	return string(rr)
}

// MakeSimpleToUnicode creates a ToUnicodeFile object for the encoding of a
// simple font.
func MakeSimpleToUnicode(data map[byte]string) *ToUnicodeFile {
	g := tuEncSimple(data)
	ee, err := dag.ShortestPath(g, 256)
	if err != nil {
		panic("unreachable")
	}

	res := &ToUnicodeFile{
		CodeSpaceRange: charcode.Simple,
	}
	code := 0
	for _, e := range ee {
		switch e.tp {
		case 1:
			res.Singles = append(res.Singles, ToUnicodeSingle{
				Code:  []byte{byte(code)},
				Value: data[byte(code)],
			})
		case 2:
			res.Ranges = append(res.Ranges, ToUnicodeRange{
				First:  []byte{byte(code)},
				Last:   []byte{byte(code + int(e.num) - 1)},
				Values: []string{data[byte(code)]},
			})
		case 3:
			values := make([]string, int(e.num))
			for i := 0; i < int(e.num); i++ {
				values[i] = data[byte(code+i)]
			}
			res.Ranges = append(res.Ranges, ToUnicodeRange{
				First:  []byte{byte(code)},
				Last:   []byte{byte(code + int(e.num) - 1)},
				Values: values,
			})
		}
		code = g.To(code, e)
	}
	return res
}

// edge types:
//
//	0 = skip unmapped codes
//	1 = use a single
//	2 = use a range with increments
//	3 = use a range with a list
type edge struct {
	tp  byte
	num uint16
}

type tuEncSimple map[byte]string

func (g tuEncSimple) has(code int) bool {
	if code < 0 || code >= 256 {
		return false
	}
	_, ok := g[byte(code)]
	return ok
}

func (g tuEncSimple) AppendEdges(ee []edge, v int) []edge {
	gapLen := 0
	for v+gapLen < 256 && !g.has(v+gapLen) {
		gapLen++
	}
	if gapLen > 0 {
		return append(ee, edge{0, uint16(gapLen)})
	}

	runLen := 1
	current := g[byte(v)]
	for len(current) > 0 && g.has(v+runLen) {
		u16 := utf16.Encode([]rune(current))
		if u16[len(u16)-1] == 0xFFFF {
			break
		}
		u16[len(u16)-1]++
		next := string(utf16.Decode(u16))
		if g[byte(v+runLen)] != next {
			break
		}

		current = next
		runLen++
	}
	if runLen == 1 {
		ee = append(ee, edge{1, 1})
	} else {
		ee = append(ee, edge{2, uint16(runLen)})
	}
	if !g.has(v + runLen) {
		return ee
	}

	for g.has(v + runLen) {
		runLen++
	}
	return append(ee, edge{3, uint16(runLen)})
}

func (g tuEncSimple) Length(v int, e edge) int {
	switch e.tp {
	case 1:
		return 2
	case 2:
		return 3
	case 3:
		return 3 + int(e.num)
	default:
		return 0
	}
}

func (g tuEncSimple) To(v int, e edge) int {
	return v + int(e.num)
}
