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
	"maps"
	"os"
	"slices"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/graphics/color"
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(fname string) error {
	pdfVersion := pdf.V2_0
	base := uint16(0xF000)

	fb, err := NewFontBuilder()
	if err != nil {
		return err
	}

	labelFont, err := gofont.Mono.NewSimple(nil)
	if err != nil {
		return err
	}
	textFont, err := gofont.Regular.NewSimple(nil)
	if err != nil {
		return err
	}

	black := color.DeviceGray(0)
	gray := color.DeviceGray(0.65)
	blue := color.DeviceRGB{0, 0, 0.8}

	out, err := NewOutput(fname, pdfVersion)
	if err != nil {
		return err
	}

	// title page
	out.Println(black, textFont,
		"TrueType Character Encoding Test")
	out.Println()
	out.Println("This document determines which method a PDF viewer uses to map")
	out.Println("character codes to glyphs in simple TrueType fonts.  Each test")
	out.Println("uses a specially crafted font where different methods produce")
	out.Println("different three-digit codes.  Different PDF viewers may show")
	out.Println("different codes, depending on the glyph selection algorithm used.")
	out.Println()
	out.Println("Read the code your viewer shows, then follow the matching line")
	out.Println("to the next test until you reach a result (shown in blue).")
	out.Println()
	out.Println("Given a single-byte code c, the methods are:")
	out.Println()
	out.Println(labelFont, "A", textFont,
		": look up c directly in a (1,0) cmap subtable.")
	out.Println(labelFont, "B", textFont,
		": look up c+0xF000 in a (3,0) cmap subtable.")
	out.Println(labelFont, "C", textFont,
		": use the PDF encoding to map c to a glyph name,")
	out.Println("    map the name to a Mac OS Roman code,")
	out.Println("    then look up that code in a (1,0) cmap subtable.")
	out.Println(labelFont, "D", textFont,
		": use the PDF encoding to map c to a glyph name,")
	out.Println("    map the name to Unicode via the Adobe Glyph List,")
	out.Println("    then look up the character in a (3,1) cmap subtable.")
	out.Println(labelFont, "E", textFont,
		": use the PDF encoding to map c to a glyph name,")
	out.Println("    then look up the name in the post table.")
	out.Println()
	out.Println("Four decision trees follow, one for each combination of the")
	out.Println("Symbolic flag and the Encoding entry in the PDF font dictionary.")

	for _, useEncoding := range []bool{true, false} {
		for _, useSymbolic := range []bool{true, false} {
			out.NewPage()
			err := writeTree(out, fb, labelFont, textFont,
				black, gray, blue, useSymbolic, useEncoding, base)
			if err != nil {
				return err
			}
		}
	}

	return out.Close()
}

type node struct {
	tag     string
	choices []string
}

func writeTree(out *output, fb *fontBuilder,
	labelFont, textFont font.Instance,
	black, gray, blue color.Color,
	useSymbolic, useEncoding bool,
	base uint16,
) error {
	var methods string
	if useEncoding {
		methods = "ABCDE"
	} else {
		methods = "AB"
	}

	out.Println(black, textFont,
		fmt.Sprintf("symbolic=%t, encoding=%t",
			useSymbolic, useEncoding))
	out.Println()

	var candidates []string
	for _, subset := range subsets(methods) {
		// A and C both use the (1,0) cmap, so a viewer cannot
		// support both independently.  Exclude orderings that
		// contain both.
		if strings.ContainsRune(subset, 'A') && strings.ContainsRune(subset, 'C') {
			continue
		}
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
			out.Println(
				black, labelFont, cur.tag,
				textFont, ": order ", blue, cur.choices[0])
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

			// at most one candidate may fall through to "XXX",
			// otherwise those candidates would be indistinguishable
			if len(m['X']) > 1 {
				continue
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
		cc := slices.Sorted(maps.Keys(bestM))
		var leaves []any
		for _, c := range cc {
			choices := bestM[c]
			if c == 'X' {
				if len(choices) == 1 {
					leaves = append(leaves, black, ", XXX=", blue, choices[0])
				}
				continue
			}

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
			default:
				panic(fmt.Sprintf("unexpected selector type %q", c))
			}
			if len(choices) == 1 {
				leaves = append(leaves, black, ", "+tag+"=", blue, choices[0])
			} else {
				todo = append(todo, node{tag, choices})
			}
		}
		X, err := fb.BuildFont(enc)
		if err != nil {
			return err
		}

		rev := make(map[string][]byte)
		rev[enc.cmap_1_0] = append(rev[enc.cmap_1_0], 'A')
		rev[enc.cmap_3_0] = append(rev[enc.cmap_3_0], 'B')
		rev[enc.cmap_1_0_enc] = append(rev[enc.cmap_1_0_enc], 'C')
		rev[enc.cmap_3_1] = append(rev[enc.cmap_3_1], 'D')
		rev[enc.post] = append(rev[enc.post], 'E')

		var tags []string
		if len(cur.choices) > 6 {
			tags = append(tags, fmt.Sprintf("%d orders", len(cur.choices)))
		} else {
			tags = append(tags, strings.Join(cur.choices, "|"))
		}
		keys := slices.Sorted(maps.Keys(rev))
		for _, tag := range keys {
			if len(tag) > 0 {
				mm := rev[tag]
				tags = append(tags, fmt.Sprintf("%s:%s", tag, string(mm)))
			}
		}

		aa := []any{
			black,
			labelFont, cur.tag,
			textFont, ": go to ",
			X, pdf.String(markerString),
			textFont, gray, " " + strings.Join(tags, ", "),
		}
		aa = append(aa, leaves...)

		out.Println(aa...)
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
		var choice strings.Builder
		for j, char := range s {
			if bits&(1<<j) != 0 {
				choice.WriteString(string(char))
			}
		}
		res = append(res, choice.String())
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
