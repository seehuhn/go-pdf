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

package main

import (
	"fmt"
	"log"
	"slices"
	"strings"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/graphics/color"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	version := pdf.V1_2
	useSymbolic := true
	useEncoding := true
	base := uint16(0x0000)

	var methods string
	if useEncoding {
		methods = "ABCDE"
	} else {
		methods = "AB"
	}

	fb, err := NewFontBuilder()
	if err != nil {
		return err
	}

	labelFont, err := gofont.Mono.New(nil)
	if err != nil {
		return err
	}
	textFont, err := gofont.Regular.New(nil)
	if err != nil {
		return err
	}

	cs, err := color.CalRGB(color.WhitePointD65, nil, nil, nil)
	if err != nil {
		return err
	}
	black := cs.New(0, 0, 0)
	gray := cs.New(0.7, 0.7, 0.7)
	blue := cs.New(0, 0, 0.8)

	out, err := NewOutput("test.pdf", version)
	if err != nil {
		return err
	}
	out.Println(black, textFont,
		fmt.Sprintf("PDF version %s, symbolic=%t, encoding=%t, base=0x%04X",
			version, useSymbolic, useEncoding, base))
	out.Println()

	type node struct {
		tag     string
		choices []string
	}
	var candidates []string
	for _, subset := range subsets(methods) {
		candidates = append(candidates, permutations(subset)...)
	}
	todo := []node{
		{"000", candidates},
	}

	nextNodeID := 1
	for len(todo) > 0 {
		cur := todo[0]
		k := copy(todo, todo[1:])
		todo = todo[:k]

		if len(cur.choices) == 1 {
			// no choice to make, this is a leaf node
			out.Println(
				black, labelFont, cur.tag,
				textFont, ": the order is ", blue, cur.choices[0])
			continue
		}

		// Try all possible splits, and choose the best one according to
		// the following criteria:
		//   - reduce the size of the largest child node as much as possible
		//   - then maximize the size of the smallest child node
		//   - then use the fewest number of selectors
		var bestM map[byte][]string
		bestMax := len(cur.choices) + 1
		bestMin := -1
		for _, sel := range subsets(methods) {
			m := make(map[byte][]string)
			for _, order := range cur.choices {
				x := findFirstMatch(order, sel)
				m[x] = append(m[x], order)
			}

			min := len(cur.choices) + 1
			max := -1
			for c, v := range m {
				if c == 'X' {
					continue
				}
				if len(v) < min {
					min = len(v)
				}
				if len(v) > max {
					max = len(v)
				}
			}

			if max < bestMax || (max == bestMax && min > bestMin) || (max == bestMax && min == bestMin && len(m) < len(bestM)) {
				bestM = m
				bestMin = min
				bestMax = max
			}
		}

		if bestMax == len(cur.choices) {
			out.Println(black,
				labelFont, cur.tag,
				textFont, fmt.Sprintf(": %d orders are possible", bestMax))
			continue
		}

		enc := &encInfo{
			useEncoding: useEncoding,
			useSymbolic: useSymbolic,
			base:        base,
		}
		cc := maps.Keys(bestM)
		slices.Sort(cc)
		var xxx []string
		for _, c := range cc {
			choices := bestM[c]
			tag := fmt.Sprintf("%03d", nextNodeID)
			nextNodeID++

			switch c {
			case 'A':
				enc.cmap_1_0 = tag
			case 'B':
				enc.cmap_3_0 = tag
			case 'C':
				enc.cmap_1_0_enc = tag
			case 'D':
				enc.cmap_3_1 = tag
			case 'E':
				enc.post = tag
			case 'X':
				xxx = choices
			default:
				panic(fmt.Sprintf("unexpected selector type %q", c))
			}
			todo = append(todo, node{tag, choices})
		}
		X, err := fb.Build(enc)
		if err != nil {
			return err
		}
		sel := string(cc)

		var tags []string
		tags = append(tags, "sel="+sel)
		if len(xxx) <= 6 {
			tags = append(tags, "XXX="+strings.Join(xxx, "|"))
		}

		if len(cur.choices) > 6-len(xxx) {
			tags = append(tags, fmt.Sprintf("%d orders", len(cur.choices)))
		} else {
			tags = append(tags, strings.Join(cur.choices, "|"))
		}

		slices.Sort(cc)
		out.Println(black,
			labelFont, cur.tag,
			textFont, ": go to ",
			X, pdf.String(markerString),
			textFont, gray, " "+strings.Join(tags, ", "))
	}

	err = out.Close()
	if err != nil {
		return err
	}
	return nil
}

func permutations(s string) []string {
	if len(s) <= 1 {
		return []string{s}
	}

	var result []string
	for i, char := range s {
		rest := s[:i] + s[i+1:]
		for _, perm := range permutations(rest) {
			result = append(result, string(char)+perm)
		}
	}
	return result
}

// subsets returns all non-empty subsets of the input string.
func subsets(s string) []string {
	var res []string
	for bits := 1; bits < 1<<len(s); bits++ {
		var choice string
		for j, char := range s {
			if bits&(1<<j) != 0 {
				choice += string(char)
			}
		}
		res = append(res, choice)
	}
	return res
}

func findFirstMatch(order, sel string) byte {
	for _, c := range order {
		if strings.ContainsRune(sel, c) {
			return byte(c)
		}
	}
	return 'X' // no selector matched
}
