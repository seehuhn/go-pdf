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

package boxes

import (
	"fmt"
	"math"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/sfnt/glyph"
)

type lineBreakGraph struct {
	hlist     []interface{}
	textWidth float64
	leftSkip  *glue
	rightSkip *glue
}

// Edge returns the outgoing edges of the given vertex.
func (g *lineBreakGraph) Edges(v *breakNode) []int {
	var res []int

	totalWidth := 0.0
	glyphsSeen := false
	for pos := v.pos + 1; pos < len(g.hlist); pos++ {
		switch h := g.hlist[pos].(type) {
		case *glue:
			if glyphsSeen {
				res = append(res, pos)
				glyphsSeen = false
			}
			totalWidth += h.Length - h.Minus.Val
		case *hGlyphs:
			glyphsSeen = true
			totalWidth += h.width
		default:
			panic(fmt.Sprintf("unexpected type %T in horizontal mode list", h))
		}
		if totalWidth > g.textWidth && len(res) > 0 {
			break
		}
	}

	if totalWidth <= g.textWidth {
		res = append(res, len(g.hlist))
	}
	if len(res) == 0 {
		panic("no edges found")
	}

	return res
}

func (g *lineBreakGraph) getRelStretch(v *breakNode, e int) float64 {
	width := &glue{}
	width = width.Add(g.leftSkip)
	for pos := v.pos; pos < e; pos++ {
		switch h := g.hlist[pos].(type) {
		case *glue:
			width = width.Add(h)
		case *hGlyphs:
			width.Length += h.width
		default:
			panic(fmt.Sprintf("unexpected type %T in horizontal mode list", h))
		}
	}
	width = width.Add(g.rightSkip)

	absStretch := g.textWidth - width.Length

	var relStretch float64
	if absStretch >= 0 {
		if width.Plus.Level == 0 {
			relStretch = absStretch / width.Plus.Val
		}
	} else {
		if width.Minus.Level > 0 {
			panic("infinite shrinkage")
		}
		relStretch = absStretch / width.Minus.Val
	}
	return relStretch
}

// Length returns the cost of adding a line break at e.
func (g *lineBreakGraph) Length(v *breakNode, e int) float64 {
	q := g.getRelStretch(v, e)

	cost := 0.0
	if q < -1 {
		cost += 1000
	} else {
		cost += 100 * q * q
	}
	if v.lineNo > 0 && math.Abs(q-v.prevRelStretch) > 0.1 {
		cost += 10
	}
	return cost * cost
}

// To returns the endpoint of a edge e starting at vertex v.
func (g *lineBreakGraph) To(v *breakNode, e int) *breakNode {
	pos := e
	for pos < len(g.hlist) && discardible(g.hlist[pos]) {
		pos++
	}
	return &breakNode{
		lineNo:         v.lineNo + 1,
		pos:            pos,
		prevRelStretch: g.getRelStretch(v, e),
	}
}

func discardible(h interface{}) bool {
	switch h.(type) {
	case *glue:
		return true
	case *hGlyphs:
		return false
	default:
		panic(fmt.Sprintf("unexpected type %T in horizontal mode list", h))
	}
}

type breakNode struct {
	lineNo         int
	pos            int
	prevRelStretch float64
}

type hGlyphs struct {
	glyphs   glyph.Seq
	font     *font.Font
	fontSize float64
	width    float64
}
