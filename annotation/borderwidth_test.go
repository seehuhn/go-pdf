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

package annotation

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestEffectiveBorderWidth(t *testing.T) {
	cases := []struct {
		name string
		a    Annotation
		want float64
	}{
		{
			// nothing was asked for, so no border is drawn
			"neither",
			&Square{},
			0,
		},
		{
			"border only",
			&Square{Common: Common{Border: &Border{Width: 2}}},
			2,
		},
		{
			"the PDF default border",
			&Square{Common: Common{Border: PDFDefaultBorder}},
			1,
		},
		{
			"style only",
			&Square{BorderStyle: &BorderStyle{Width: 3}},
			3,
		},
		{
			// the border array is ignored whenever a style is present
			"style wins over border",
			&Square{
				Common:      Common{Border: &Border{Width: 2}},
				BorderStyle: &BorderStyle{Width: 3},
			},
			3,
		},
		{
			// a style asking for width 0 is not overridden by the array
			"style suppresses the border",
			&Square{
				Common:      Common{Border: &Border{Width: 2}},
				BorderStyle: &BorderStyle{Width: 0},
			},
			0,
		},
		{
			// a type without a border style dictionary uses the array alone
			"no style dictionary",
			&Text{Common: Common{Border: &Border{Width: 4}}},
			4,
		},
		{
			"no style dictionary and no border",
			&Text{},
			0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveBorderWidth(tc.a); got != tc.want {
				t.Errorf("EffectiveBorderWidth = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEffectiveBorderStyle(t *testing.T) {
	cases := []struct {
		name string
		a    Annotation
		want pdf.Name
	}{
		{"neither", &Square{}, "S"},
		{"plain border array", &Square{Common: Common{Border: PDFDefaultBorder}}, "S"},
		{
			// a border array has no style entry, so a dash pattern is the
			// only way it can ask for anything but a solid border
			"dashed border array",
			&Square{Common: Common{Border: &Border{Width: 1, DashArray: []float64{2, 2}}}},
			"D",
		},
		{"style with no style name", &Square{BorderStyle: &BorderStyle{Width: 1}}, "S"},
		{"beveled style", &Square{BorderStyle: &BorderStyle{Width: 1, Style: "B"}}, "B"},
		{
			// the array is ignored whenever a style is present, so its dashes
			// cannot make a solid style dashed
			"solid style over dashed array",
			&Square{
				Common:      Common{Border: &Border{Width: 1, DashArray: []float64{2, 2}}},
				BorderStyle: &BorderStyle{Width: 1, Style: "S"},
			},
			"S",
		},
		{"no style dictionary", &Text{}, "S"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveBorderStyle(tc.a); got != tc.want {
				t.Errorf("EffectiveBorderStyle = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEffectiveBorderDash(t *testing.T) {
	cases := []struct {
		name string
		a    Annotation
		want []float64
	}{
		{"neither", &Square{}, nil},
		{"solid border array", &Square{Common: Common{Border: PDFDefaultBorder}}, nil},
		{
			"dashed border array",
			&Square{Common: Common{Border: &Border{Width: 1, DashArray: []float64{2, 3}}}},
			[]float64{2, 3},
		},
		{
			"dashed style",
			&Square{BorderStyle: &BorderStyle{Width: 1, Style: "D", DashArray: []float64{4, 1}}},
			[]float64{4, 1},
		},
		{
			// the array is ignored whenever a style is present
			"solid style over dashed array",
			&Square{
				Common:      Common{Border: &Border{Width: 1, DashArray: []float64{2, 3}}},
				BorderStyle: &BorderStyle{Width: 1, Style: "S"},
			},
			nil,
		},
		{
			"no style dictionary",
			&Text{Common: Common{Border: &Border{Width: 1, DashArray: []float64{5}}}},
			[]float64{5},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveBorderDash(tc.a); !slices.Equal(got, tc.want) {
				t.Errorf("EffectiveBorderDash = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSetBorderWidth checks that a width set through [SetBorderWidth] is the
// width [EffectiveBorderWidth] reads back, whichever of the two places the
// annotation was carrying its border in, and that the annotation is never
// left carrying both.
func TestSetBorderWidth(t *testing.T) {
	cases := []struct {
		name string
		a    Annotation
		// wantStyle is whether the width should land in the border style
		// dictionary rather than in the border array
		wantStyle bool
	}{
		{"neither", &Square{}, true},
		{"border array only", &Square{Common: Common{Border: &Border{Width: 2}}}, false},
		{"style only", &Square{BorderStyle: &BorderStyle{Width: 3}}, true},
		{"a line", &Line{}, true},
		{"a circle", &Circle{}, true},
		{"a polygon", &Polygon{}, true},
		{"a polyline", &PolyLine{}, true},
		{"a free text box", &FreeText{}, true},
		{"a widget", &Widget{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			SetBorderWidth(tc.a, 6, pdf.V2_0)

			if got := EffectiveBorderWidth(tc.a); got != 6 {
				t.Errorf("effective width = %v, want 6", got)
			}
			bs, ok := tc.a.(borderStyled)
			if !ok {
				t.Fatal("the case is set up wrongly: the type carries no border style")
			}
			hasStyle := bs.getBorderStyle() != nil
			if hasStyle != tc.wantStyle {
				t.Errorf("the width went into the style = %v, want %v",
					hasStyle, tc.wantStyle)
			}
			// the two are mutually exclusive: an annotation carrying both
			// cannot be written at all
			if hasStyle && tc.a.GetCommon().Border != nil {
				t.Error("the border array was left in place beside the style")
			}
		})
	}
}

// TestSetBorderWidthBelowTheStyleVersion checks that a width set on a type
// whose style dictionary the file is too old to carry goes into the border
// array instead.  A link annotation's BS entry arrived in PDF 1.6, where the
// type dates from PDF 1.0, so a style written at an earlier version would
// leave the annotation unwritable.
func TestSetBorderWidthBelowTheStyleVersion(t *testing.T) {
	for _, tc := range []struct {
		version   pdf.Version
		wantStyle bool
	}{
		{pdf.V1_0, false},
		{pdf.V1_5, false},
		{pdf.V1_6, true},
		{pdf.V2_0, true},
	} {
		t.Run(tc.version.String(), func(t *testing.T) {
			a := &Link{}
			SetBorderWidth(a, 2, tc.version)

			if got := EffectiveBorderWidth(a); got != 2 {
				t.Errorf("effective width = %v, want 2", got)
			}
			if got := a.BorderStyle != nil; got != tc.wantStyle {
				t.Errorf("the width went into the style = %v, want %v",
					got, tc.wantStyle)
			}

			w, _ := memfile.NewPDFWriter(t, tc.version, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := a.Encode(rm); err != nil {
				t.Errorf("the annotation cannot be written: %v", err)
			}
		})
	}
}

// TestSetBorderWidthZeroVersion checks that a caller with no version to give
// gets the border array, which every version of the format has.
func TestSetBorderWidthZeroVersion(t *testing.T) {
	a := &Link{}
	SetBorderWidth(a, 2, 0)

	if a.BorderStyle != nil {
		t.Error("the width went into a style dictionary the file may not carry")
	}
	if got := EffectiveBorderWidth(a); got != 2 {
		t.Errorf("effective width = %v, want 2", got)
	}
}

// TestSetBorderWidthKeepsTheStyle checks that only the width changes: a
// dashed border stays dashed, and the corner radii of a border array are
// not silently dropped for a type which has nowhere else to put them.
func TestSetBorderWidthKeepsTheStyle(t *testing.T) {
	a := &Square{BorderStyle: &BorderStyle{
		Width: 1, Style: "D", DashArray: []float64{3, 2},
	}}
	SetBorderWidth(a, 4, pdf.V2_0)

	if a.BorderStyle.Width != 4 {
		t.Errorf("width = %v, want 4", a.BorderStyle.Width)
	}
	if a.BorderStyle.Style != "D" {
		t.Errorf("style = %q, want D", a.BorderStyle.Style)
	}
	if !slices.Equal(a.BorderStyle.DashArray, []float64{3, 2}) {
		t.Errorf("dash array = %v, want [3 2]", a.BorderStyle.DashArray)
	}
}

// TestSetBorderWidthKeepsTheArray checks that a border the annotation was
// already carrying in its array keeps its corner radii and dashes, which a
// style dictionary has no room for.
func TestSetBorderWidthKeepsTheArray(t *testing.T) {
	a := &Square{Common: Common{Border: &Border{
		HCornerRadius: 3, VCornerRadius: 4, Width: 1, DashArray: []float64{5, 2},
	}}}
	SetBorderWidth(a, 7, pdf.V2_0)

	if a.BorderStyle != nil {
		t.Fatal("the border moved into a style, losing the corner radii")
	}
	want := &Border{
		HCornerRadius: 3, VCornerRadius: 4, Width: 7, DashArray: []float64{5, 2},
	}
	if diff := cmp.Diff(want, a.Border); diff != "" {
		t.Errorf("border changed beyond its width (-want +got):\n%s", diff)
	}
}

// TestSetBorderWidthCopiesTheArray checks that a border array shared between
// annotations is not changed underneath the ones which were not asked about.
func TestSetBorderWidthCopiesTheArray(t *testing.T) {
	shared := &Border{Width: 1}
	a := &Text{Common: Common{Border: shared}}
	b := &Text{Common: Common{Border: shared}}

	SetBorderWidth(a, 5, pdf.V2_0)

	if got := EffectiveBorderWidth(a); got != 5 {
		t.Errorf("effective width = %v, want 5", got)
	}
	if got := EffectiveBorderWidth(b); got != 1 {
		t.Errorf("the other annotation changed too: width = %v, want 1", got)
	}
	if shared.Width != 1 {
		t.Errorf("the shared border was modified: width = %v, want 1", shared.Width)
	}
}

// TestSetBorderWidthOnAPlainType checks that a type with no border style
// dictionary takes the width in its border array, which is the only place
// it has for one.
func TestSetBorderWidthOnAPlainType(t *testing.T) {
	a := &Text{}
	SetBorderWidth(a, 3, pdf.V2_0)

	if got := EffectiveBorderWidth(a); got != 3 {
		t.Errorf("effective width = %v, want 3", got)
	}
	if a.Border == nil {
		t.Fatal("the width went nowhere")
	}
}

// TestSetBorderWidthZeroRemovesIt checks that a width of 0 leaves the
// annotation with no border at all, in either place: a border missing at
// this point is one the file asked to be left undrawn.
func TestSetBorderWidthZeroRemovesIt(t *testing.T) {
	for _, tc := range []struct {
		name string
		a    *Square
	}{
		{"both", &Square{
			Common:      Common{Border: &Border{Width: 2}},
			BorderStyle: &BorderStyle{Width: 3},
		}},
		{"array only", &Square{Common: Common{Border: &Border{Width: 2}}}},
		{"style only", &Square{BorderStyle: &BorderStyle{Width: 3}}},
		{"neither", &Square{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			SetBorderWidth(tc.a, 0, pdf.V2_0)

			if got := EffectiveBorderWidth(tc.a); got != 0 {
				t.Errorf("effective width = %v, want 0", got)
			}
			if tc.a.Border != nil || tc.a.BorderStyle != nil {
				t.Errorf("border = %v, style = %v, want neither",
					tc.a.Border, tc.a.BorderStyle)
			}
		})
	}
}
