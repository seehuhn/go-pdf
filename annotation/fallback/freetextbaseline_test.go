// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package fallback

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

// TestFreeTextLastLineDescenderInsideClip checks that the descenders of the
// last line of a text box lie inside the rectangle the text is clipped to.
//
// A box is as tall as its lines: the content area holds one line height per
// line.  With the first baseline a full font size below the top of the
// content area, what is left below the last baseline is the difference
// between the leading and 1, which is less than either font descends, so the
// bottom line loses the tails of its letters.  The first baseline therefore
// sits one ascent below the top instead, which leaves the rest of the leading
// where the descenders need it.
func TestFreeTextLastLineDescenderInsideClip(t *testing.T) {
	const size = 12

	for _, tc := range []struct {
		name        string
		typewriter  bool
		borderWidth float64
		contents    string
	}{
		{"text box", false, 1, "the modern world is not too far, for wombats wiggling in a jar"},
		{"typewriter", true, 0, "for wombats\nwiggling in a jar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newGen(t, pdf.V2_0)

			a := &annotation.FreeText{
				Common: annotation.Common{
					Contents: tc.contents,
				},
				DefaultAppearance: "/F " + strconv.Itoa(size) + " Tf 0 0 0 rg",
			}
			if tc.typewriter {
				a.Markup.Intent = annotation.FreeTextIntentTypeWriter
			}
			if tc.borderWidth > 0 {
				a.Border = &annotation.Border{Width: tc.borderWidth}
			}

			// the box a viewer lays two lines out in: one line height per
			// line, plus the border and the padding on both sides
			F := g.freeTextFont(a)
			lineHeight := pdf.Round(F.GetGeometry().Leading*size, 2)
			inset := tc.borderWidth + freeTextPadding
			height := 2*lineHeight + 2*inset
			a.Rect = pdf.Rectangle{LLx: 0, LLy: 0, URx: 180, URy: height}

			if err := g.AddAppearance(a); err != nil {
				t.Fatal(err)
			}
			body := appearanceContent(t, a)

			clip := clipBottomOf(t, body)
			last := lastBaselineOf(t, body)
			room := last - clip
			need := -F.GetGeometry().Descent * size
			if room < need {
				t.Errorf("only %g below the last baseline, the descenders need %g",
					room, need)
			}

			// and the first line is not pushed out of the top either
			first := firstBaselineOf(t, body)
			top := height - inset
			if ascender := first + F.GetGeometry().Ascent*size; ascender > top+1e-9 {
				t.Errorf("the first line reaches %g, the content area ends at %g",
					ascender, top)
			}
		})
	}
}

// TestFreeTextAscentsUsedForFirstBaseline pins the ascents the generator
// places the first baseline by.
//
// A viewer which lays the same text out has to put its first baseline in the
// same place, and where it cannot read the metrics out of the font it holds
// them as constants.  This test is what those constants are copied from: a
// change here has to be carried over to them.
func TestFreeTextAscentsUsedForFirstBaseline(t *testing.T) {
	g := newGen(t, pdf.V2_0)

	cases := []struct {
		name string
		a    *annotation.FreeText
		want float64
	}{
		{"text box", &annotation.FreeText{}, 0.729},
		{
			"typewriter",
			&annotation.FreeText{Markup: annotation.Markup{Intent: annotation.FreeTextIntentTypeWriter}},
			0.825,
		},
	}
	for _, c := range cases {
		got := g.freeTextFont(c.a).GetGeometry().Ascent
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: ascent %g, want %g", c.name, got, c.want)
		}
	}
}

// clipBottomOf returns the y coordinate of the bottom edge of the rectangle
// the appearance clips its text to.
func clipBottomOf(t *testing.T, content string) float64 {
	t.Helper()

	var last []string
	for line := range strings.SplitSeq(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 1 && fields[0] == "W" && len(last) == 5 {
			y, err := strconv.ParseFloat(last[1], 64)
			if err != nil {
				t.Fatal(err)
			}
			return y
		}
		if len(fields) == 5 && fields[4] == "re" {
			last = fields
		} else {
			last = nil
		}
	}
	t.Fatal("the appearance clips to nothing")
	return 0
}

// baselinesOf returns the y coordinates of the text lines' baselines, summed
// from the Td and TD operators that step from one to the next.
func baselinesOf(t *testing.T, content string) []float64 {
	t.Helper()

	var ys []float64
	y := 0.0
	for line := range strings.SplitSeq(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || (fields[2] != "Td" && fields[2] != "TD") {
			continue
		}
		dy, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			t.Fatal(err)
		}
		if len(ys) == 0 {
			y = dy
		} else {
			y += dy
		}
		ys = append(ys, y)
	}
	if len(ys) == 0 {
		t.Fatal("the appearance draws no text")
	}
	return ys
}

func firstBaselineOf(t *testing.T, content string) float64 {
	t.Helper()
	return baselinesOf(t, content)[0]
}

func lastBaselineOf(t *testing.T, content string) float64 {
	t.Helper()
	ys := baselinesOf(t, content)
	return ys[len(ys)-1]
}
