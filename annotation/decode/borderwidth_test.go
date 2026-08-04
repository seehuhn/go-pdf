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

package decode

import (
	"io"
	"maps"
	"slices"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// borderArrays and borderStyles are the ways a file can describe a border,
// including ones no Go value can express: a dictionary may carry both entries,
// and either may hold a width the writer would refuse.
var (
	borderArrays = map[string]pdf.Object{
		"absent":   nil,
		"[0 0 0]":  pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(0)},
		"[0 0 1]":  pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(1)},
		"[0 0 2]":  pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(2)},
		"[0 0 -1]": pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(-1)},
		"dashed": pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(1),
			pdf.Array{pdf.Number(2), pdf.Number(3)}},
		"rounded": pdf.Array{pdf.Number(2), pdf.Number(3), pdf.Number(1)},
	}
	borderStyles = map[string]pdf.Object{
		"absent":        nil,
		"empty":         pdf.Dict{},
		"solid":         pdf.Dict{"S": pdf.Name("S")},
		"dashed":        pdf.Dict{"S": pdf.Name("D")},
		"dashed w/ arr": pdf.Dict{"S": pdf.Name("D"), "D": pdf.Array{pdf.Number(4), pdf.Number(1)}},
		"beveled":       pdf.Dict{"S": pdf.Name("B")},
		"unknown style": pdf.Dict{"S": pdf.Name("Z")},
		"W=0":           pdf.Dict{"W": pdf.Number(0)},
		"W=3":           pdf.Dict{"W": pdf.Number(3)},
		"W=-1":          pdf.Dict{"W": pdf.Number(-1)},
	}
)

// squareWithBorder returns a square annotation dictionary carrying the given
// border entries, leaving out the ones which are nil.
func squareWithBorder(border, style pdf.Object) pdf.Dict {
	dict := pdf.Dict{
		"Subtype": pdf.Name("Square"),
		"Rect":    &pdf.Rectangle{URx: 100, URy: 50},
	}
	if border != nil {
		dict["Border"] = border
	}
	if style != nil {
		dict["BS"] = style
	}
	return dict
}

// borderAppearance returns the content stream of the appearance synthesized
// for the annotation, as a string.  A fresh generator is used for each call, so
// that the result depends on the annotation alone.
func borderAppearance(t *testing.T, a annotation.Annotation) string {
	t.Helper()
	gen, err := fallback.NewStyle().New(pdf.V2_0)
	if err != nil {
		t.Fatal(err)
	}
	if err := gen.AddAppearance(a); err != nil {
		t.Fatal(err)
	}
	ap := annotation.Resolve(a.GetCommon(), appearance.Normal)
	if ap == nil || ap.Content == nil {
		return ""
	}
	r, err := ap.Content.RawBytes()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestBorderSurvivesRoundTrip checks that neither the border a reader recovers
// nor the appearance synthesized from it changes when the annotation is
// written out and read back.  The border array and the border style dictionary
// can each stand in for the other's absence, and a file may carry both, so the
// value the second read sees is often spelled differently from the first;
// what has to survive is the border it describes.
func TestBorderSurvivesRoundTrip(t *testing.T) {
	for bn, border := range borderArrays {
		for sn, style := range borderStyles {
			t.Run("Border="+bn+",BS="+sn, func(t *testing.T) {
				src, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
				first, err := Annotation(pdf.NewCursor(src), squareWithBorder(border, style), true)
				if err != nil {
					t.Fatal(err)
				}
				wantWidth := annotation.EffectiveBorderWidth(first)
				wantStyle := annotation.EffectiveBorderStyle(first)
				wantDash := annotation.EffectiveBorderDash(first)
				wantAppearance := borderAppearance(t, withoutAppearance(first))

				// PDF 1.7, where a square annotation need not carry an
				// appearance stream: the border entries are the subject here
				buf, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
				rm := pdf.NewResourceManager(buf)
				obj, err := first.Encode(rm)
				if err != nil {
					t.Fatalf("cannot write the annotation back: %v", err)
				}
				if err := rm.Close(); err != nil {
					t.Fatal(err)
				}

				second, err := Annotation(pdf.NewCursor(buf), obj, true)
				if err != nil {
					t.Fatal(err)
				}
				if got := annotation.EffectiveBorderWidth(second); got != wantWidth {
					t.Errorf("width = %v after the round trip, want %v", got, wantWidth)
				}
				if got := annotation.EffectiveBorderStyle(second); got != wantStyle {
					t.Errorf("style = %v after the round trip, want %v", got, wantStyle)
				}
				if got := annotation.EffectiveBorderDash(second); !slices.Equal(got, wantDash) {
					t.Errorf("dash = %v after the round trip, want %v", got, wantDash)
				}
				if got := borderAppearance(t, withoutAppearance(second)); got != wantAppearance {
					t.Errorf("appearance after the round trip:\n got %q\nwant %q", got, wantAppearance)
				}
			})
		}
	}
}

// withoutAppearance returns a copy of the annotation with no appearance
// dictionary, so that a fallback appearance is generated for it afresh.
func withoutAppearance(a annotation.Annotation) annotation.Annotation {
	sq := *(a.(*annotation.Square))
	sq.Appearance = nil
	return &sq
}

// TestEffectiveBorderWidthFromFile checks the width a reader recovers for the
// ways a file can describe an annotation's border, including the two which
// carry no entry: an absent border array stands for the PDF default, while an
// explicit zero width asks for no border at all.
func TestEffectiveBorderWidthFromFile(t *testing.T) {
	rect := &pdf.Rectangle{URx: 100, URy: 50}
	cases := []struct {
		name string
		dict pdf.Dict
		want float64
	}{
		{"no entries", pdf.Dict{}, 1},
		{"border [0 0 0]", pdf.Dict{
			"Border": pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(0)},
		}, 0},
		{"border [0 0 3]", pdf.Dict{
			"Border": pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(3)},
		}, 3},
		{"style with no width", pdf.Dict{"BS": pdf.Dict{"S": pdf.Name("S")}}, 1},
		{"style with width 4", pdf.Dict{"BS": pdf.Dict{"W": pdf.Number(4)}}, 4},
		{"style with width 0", pdf.Dict{"BS": pdf.Dict{"W": pdf.Number(0)}}, 0},
		{"style overrides border", pdf.Dict{
			"BS":     pdf.Dict{"W": pdf.Number(4)},
			"Border": pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(3)},
		}, 4},
		{"style width 0 overrides border", pdf.Dict{
			"BS":     pdf.Dict{"W": pdf.Number(0)},
			"Border": pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(3)},
		}, 0},
	}

	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	c := pdf.NewCursor(w)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dict := pdf.Dict{"Subtype": pdf.Name("Square"), "Rect": rect}
			maps.Copy(dict, tc.dict)
			a, err := Annotation(c, dict, true)
			if err != nil {
				t.Fatal(err)
			}
			if got := annotation.EffectiveBorderWidth(a); got != tc.want {
				t.Errorf("EffectiveBorderWidth = %v, want %v", got, tc.want)
			}
		})
	}
}
