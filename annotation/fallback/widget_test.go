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
	"io"
	"math"
	"strconv"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
)

// combWidget builds a comb text field with the given value, MaxLen and
// justification, and one widget.
func combWidget(value string, maxLen int, align pdf.TextAlign) *annotation.Widget {
	f := acroform.NewTextField("c")
	f.Ff = acroform.FieldComb
	f.MaxLen = maxLen
	f.Align = align
	f.V = pdf.TextString(value)

	return annotation.AddWidget(f, pdf.Rectangle{LLx: 0, LLy: 0, URx: 62, URy: 20})
}

// the widget's field context reflects the field's MaxLen and flags
func TestResolveWidgetFieldMaxLen(t *testing.T) {
	w := combWidget("AB", 6, pdf.TextAlignLeft)
	fld := resolveWidgetField(w)
	if fld == nil {
		t.Fatal("expected a field context")
	}
	if fld.MaxLen != 6 {
		t.Errorf("MaxLen = %d, want 6", fld.MaxLen)
	}
	if fld.Flags&acroform.FieldComb == 0 {
		t.Error("Comb flag missing from field context")
	}
}

// contentTokens returns the appearance stream of the widget's normal
// appearance as whitespace-separated tokens.
func contentTokens(t *testing.T, w *annotation.Widget) []string {
	t.Helper()
	if w.Appearance == nil || w.Appearance.Normal == nil {
		t.Fatal("no normal appearance")
	}
	r, err := w.Appearance.Normal.Content.RawBytes()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(data))
}

func countToken(tokens []string, tok string) int {
	n := 0
	for _, s := range tokens {
		if s == tok {
			n++
		}
	}
	return n
}

// firstTd returns the operands of the first "Td" operator.
func firstTd(t *testing.T, tokens []string) (x, y float64) {
	t.Helper()
	for i, s := range tokens {
		if s == "Td" && i >= 2 {
			x, err1 := strconv.ParseFloat(tokens[i-2], 64)
			y, err2 := strconv.ParseFloat(tokens[i-1], 64)
			if err1 != nil || err2 != nil {
				t.Fatalf("malformed Td operands %q %q", tokens[i-2], tokens[i-1])
			}
			return x, y
		}
	}
	t.Fatal("no Td operator found")
	return 0, 0
}

// a comb appearance uses one text object and one stroke for all cell
// dividers, and a partial value follows the field's justification.
func TestCombAppearance(t *testing.T) {
	// rect width 62, border width 1: six cells of width 10 starting at x=1
	tests := []struct {
		name      string
		align     pdf.TextAlign
		wantCellX float64
	}{
		{"left", pdf.TextAlignLeft, 1},
		{"center", pdf.TextAlignCenter, 21}, // cell (6-2)/2 = 2
		{"right", pdf.TextAlignRight, 41},   // cell 6-2 = 4
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := combWidget("AB", 6, tc.align)
			s := NewStyle()
			if err := s.AddAppearance(w); err != nil {
				t.Fatal(err)
			}
			tokens := contentTokens(t, w)

			if got := countToken(tokens, "BT"); got != 1 {
				t.Errorf("BT count = %d, want 1 (one text object)", got)
			}
			if got := countToken(tokens, "Tf"); got != 1 {
				t.Errorf("Tf count = %d, want 1 (font set once)", got)
			}
			if got := countToken(tokens, "S"); got != 1 {
				t.Errorf("S count = %d, want 1 (dividers in one stroke)", got)
			}
			// five dividers between six cells
			if got := countToken(tokens, "m"); got != 5 {
				t.Errorf("m count = %d, want 5 (one per divider)", got)
			}

			if x, _ := firstTd(t, tokens); math.Abs(x-tc.wantCellX) > 0.01 {
				t.Errorf("first cell starts at x=%g, want %g", x, tc.wantCellX)
			}
		})
	}
}

// a value longer than MaxLen is truncated to the available cells.
func TestCombOverlongValue(t *testing.T) {
	w := combWidget("ABCDEFGH", 6, pdf.TextAlignLeft)
	s := NewStyle()
	if err := s.AddAppearance(w); err != nil {
		t.Fatal(err)
	}
	tokens := contentTokens(t, w)
	if got := countToken(tokens, "Td"); got != 6 {
		t.Errorf("Td count = %d, want 6 (one per occupied cell)", got)
	}
}
