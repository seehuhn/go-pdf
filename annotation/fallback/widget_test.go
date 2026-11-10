// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/form"
)

// combWidget builds a comb text field with the given value, MaxLen and
// justification, and one widget of border width 1.
func combWidget(value string, maxLen int, align pdf.TextAlign) *annotation.Widget {
	w := borderlessCombWidget(value, maxLen, align)
	w.BorderStyle = &annotation.BorderStyle{Width: 1}
	return w
}

// borderlessCombWidget is like combWidget but leaves the widget without a
// border, which is what a widget carrying neither a border style nor a border
// array asks for.
func borderlessCombWidget(value string, maxLen int, align pdf.TextAlign) *annotation.Widget {
	f := acroform.NewTextField("c")
	f.DefaultAppearance = "/Helv 0 Tf 0 g"
	f.Flags = acroform.FieldComb
	f.MaxLen = maxLen
	f.Align = align
	f.Text = &pdf.StringOrStream{Value: pdf.Text(value)}

	return annotation.AddWidget(f, pdf.Rectangle{LLx: 0, LLy: 0, URx: 62, URy: 20})
}

// the widget's field context reflects the field's MaxLen and flags
func TestResolveWidgetFieldMaxLen(t *testing.T) {
	w := combWidget("AB", 6, pdf.TextAlignLeft)
	fld := resolveWidgetField(w, widgetOnState)
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
			s := newGen(t, pdf.V2_0)
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

// TestCombAppearanceNoBorder checks that the comb cells span the whole
// rectangle when the widget asks for no border, rather than being inset by a
// border width the annotation never requested.
func TestCombAppearanceNoBorder(t *testing.T) {
	w := borderlessCombWidget("AB", 6, pdf.TextAlignLeft)
	s := newGen(t, pdf.V2_0)
	if err := s.AddAppearance(w); err != nil {
		t.Fatal(err)
	}
	tokens := contentTokens(t, w)

	if x, _ := firstTd(t, tokens); math.Abs(x) > 0.01 {
		t.Errorf("first cell starts at x=%g, want 0", x)
	}
}

// a value longer than MaxLen is truncated to the available cells.
func TestCombOverlongValue(t *testing.T) {
	w := combWidget("ABCDEFGH", 6, pdf.TextAlignLeft)
	s := newGen(t, pdf.V2_0)
	if err := s.AddAppearance(w); err != nil {
		t.Fatal(err)
	}
	tokens := contentTokens(t, w)
	if got := countToken(tokens, "Td"); got != 6 {
		t.Errorf("Td count = %d, want 6 (one per occupied cell)", got)
	}
}

// TestWidgetTransparentChrome checks that a widget whose appearance
// characteristics ask for no background and no border colour, as an empty
// /BG or /BC array does, gets an appearance which paints neither.
func TestWidgetTransparentChrome(t *testing.T) {
	for _, shape := range []string{"text", "radio"} {
		t.Run(shape, func(t *testing.T) {
			var w *annotation.Widget
			switch shape {
			case "text":
				f := acroform.NewTextField("t")
				f.DefaultAppearance = "/Helv 0 Tf 0 g"
				w = annotation.AddWidget(f, pdf.Rectangle{LLx: 0, LLy: 0, URx: 62, URy: 20})
			case "radio":
				w = toggleWidget(t, "Off", "On", true)
			}
			w.BorderStyle = &annotation.BorderStyle{Width: 1}
			w.Style = &appearance.Characteristics{
				BackgroundColor: colorenc.Transparent,
				BorderColor:     colorenc.Transparent,
			}
			s := newGen(t, pdf.V2_0)
			if err := s.AddAppearance(w); err != nil {
				t.Fatal(err)
			}

			var f *form.Form
			if shape == "text" {
				f = w.Appearance.Normal
			} else {
				f = w.Appearance.NormalMap["Off"]
			}
			if f == nil {
				t.Fatal("no appearance")
			}
			r, err := f.Content.RawBytes()
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			data, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			tokens := strings.Fields(string(data))
			if got := countToken(tokens, "f") + countToken(tokens, "S"); got != 0 {
				t.Errorf("chrome painted %d times, want none", got)
			}
		})
	}
}

// Value names the selection; Selected is consulted only when Value is
// empty.
func TestResolveWidgetFieldChoiceValue(t *testing.T) {
	tests := []struct {
		name     string
		v        []pdf.TextString
		selected []int
		want     string
	}{
		{"Value wins over Selected", []pdf.TextString{pdf.Text("A")}, []int{1}, "A"},
		{"Selected alone", nil, []int{1}, "B"},
		{"nothing selected", nil, nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := acroform.NewChoiceField("c")
			f.Options = []acroform.ChoiceOption{{Export: "a", Display: pdf.Text("A")}, {Export: "b", Display: pdf.Text("B")}}
			f.Value = tc.v
			f.Selected = tc.selected
			w := annotation.AddWidget(f, pdf.Rectangle{URx: 100, URy: 20})
			fld := resolveWidgetField(w, widgetOnState)
			if fld == nil {
				t.Fatal("expected a field context")
			}
			if fld.Value != tc.want {
				t.Errorf("Value = %q, want %q", fld.Value, tc.want)
			}
		})
	}
}

// TestDAColorOperatorNoExponent checks that small colour components are
// written in plain decimal notation.
func TestDAColorOperatorNoExponent(t *testing.T) {
	for _, col := range []color.Color{
		color.DeviceGray(0.00001),
		color.DeviceRGB{0.00001, 0, 0.5},
		color.DeviceCMYK{0, 0, 0, 0.00002},
	} {
		s := daColorOperator(col)
		if strings.ContainsAny(s, "eE") {
			t.Errorf("%T: exponent notation in %q", col, s)
		}
	}
}

// TestParseDA checks that a default-appearance string is tokenised as a
// content-stream fragment: delimiters end tokens, NUL is white space,
// comments are ignored, and only PDF number syntax gives an operand.
func TestParseDA(t *testing.T) {
	cases := []struct {
		name string
		da   string
		size float64
		col  color.Color
	}{
		{"plain", "/Helv 12 Tf 0 0 1 rg", 12, color.DeviceRGB{0, 0, 1}},
		{"name after operator", "0 0 1 rg/Helv 12 Tf", 12, color.DeviceRGB{0, 0, 1}},
		{"NUL separator", "/Helv 12 Tf\x000 0 1 rg", 12, color.DeviceRGB{0, 0, 1}},
		{"comment", "/Helv 12 Tf 0 g %1 0 0 rg\n", 12, color.DeviceGray(0)},
		{"comment between operands", "/Helv 12 Tf 0 0%x\n1 rg", 12, color.DeviceRGB{0, 0, 1}},
		{"vertical tab is regular", "/Helv 12 Tf 0\v1 g", 12, quireInk},
		{"exponent is no number", "/Helv 1e1 Tf 1e0 g", 0, quireInk},
		{"NaN is no number", "/Helv 12 Tf NaN g", 12, quireInk},
		{"gray", "/Helv 9.5 Tf 0.5 g", 9.5, color.DeviceGray(0.5)},
		{"cmyk", "/Helv 12 Tf 0 1 1 0 k", 12, color.DeviceCMYK{0, 1, 1, 0}},
		{"last colour wins", "1 g /Helv 12 Tf 0 0 1 rg", 12, color.DeviceRGB{0, 0, 1}},
		{"out of range clipped", "/Helv 12 Tf 2 -1 0.5 rg", 12, color.DeviceRGB{1, 0, 0.5}},
		{"too few operands", "/Helv 12 Tf 0 1 rg", 12, quireInk},
		{"font operand not a name", "(Helv) 12 Tf", 0, quireInk},
		{"negative size", "/Helv -12 Tf", 0, quireInk},
		{"empty", "", 0, quireInk},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			size, col := parseDA(c.da)
			if size != c.size {
				t.Errorf("parseDA(%q) size = %v, want %v", c.da, size, c.size)
			}
			if col != c.col {
				t.Errorf("parseDA(%q) colour = %v, want %v", c.da, col, c.col)
			}
		})
	}
}
