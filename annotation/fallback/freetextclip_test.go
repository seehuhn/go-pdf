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

// TestFreeTextClipMatchesContentWidth checks that the text of a FreeText
// annotation is wrapped to the width it is clipped to.
//
// A viewer which lays the same text out has to derive that width from the
// annotation itself, and the only right derivation is the one the generator
// uses: the rectangle narrowed by the border width and the content padding on
// both sides.  A viewer which forgets the border wraps its lines wider than
// the clip and the ends of them are cut off on the page -- letters missing
// from text which looked complete while it was being typed.
func TestFreeTextClipMatchesContentWidth(t *testing.T) {
	const contents = "a line long enough to need wrapping in a narrow box"

	for _, borderWidth := range []float64{0, 1, 3} {
		gen, err := NewStyle().New(pdf.V2_0)
		if err != nil {
			t.Fatal(err)
		}
		a := &annotation.FreeText{
			Common: annotation.Common{
				Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 180, URy: 100},
				Contents: contents,
			},
			DefaultAppearance: "/Helv 12 Tf 0 0 0 rg",
		}
		if borderWidth > 0 {
			a.Border = &annotation.Border{Width: borderWidth}
		}
		if err := gen.AddAppearance(a); err != nil {
			t.Fatal(err)
		}

		clip := clipWidthOf(t, appearanceContent(t, a))
		want := a.Rect.Dx() - 2*annotation.EffectiveBorderWidth(a) - 2*freeTextPadding
		if math.Abs(clip-want) > 1e-9 {
			t.Errorf("border %g: clipped to %g, Rect less border and padding is %g",
				borderWidth, clip, want)
		}

		// and the lines a viewer gets for that width really do fit in it
		lines, err := NewStyle().Lines(contents, clip)
		if err != nil {
			t.Fatal(err)
		}
		F := gen.ContentFont()
		for _, line := range lines {
			w := F.Layout(nil, freeTextFontSize, contents[line.Start:line.End]).TotalWidth()
			if w > clip+1e-9 {
				t.Errorf("border %g: line %q is %g wide, clipped at %g",
					borderWidth, contents[line.Start:line.End], w, clip)
			}
		}
	}
}

// clipWidthOf returns the width of the rectangle the appearance clips its
// text to: the rectangle the clipping path is built from, which is the one
// followed by "W".  A bordered box draws a second rectangle, its outline,
// which is stroked instead.
func clipWidthOf(t *testing.T, content string) float64 {
	t.Helper()

	var last []string
	for line := range strings.SplitSeq(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 1 && fields[0] == "W" && len(last) == 5 {
			w, err := strconv.ParseFloat(last[2], 64)
			if err != nil {
				t.Fatal(err)
			}
			return w
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
