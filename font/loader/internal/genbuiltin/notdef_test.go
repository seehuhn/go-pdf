// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyf"
)

func TestMakeNotdefBox(t *testing.T) {
	// default advance, cap height given
	box := makeNotdefBox(1000, 0, 680)
	want := notdefBox{
		advance: 600,
		outer:   funit.Rect16{LLx: 50, LLy: 0, URx: 550, URy: 680},
		inner:   funit.Rect16{LLx: 100, LLy: 50, URx: 500, URy: 630},
	}
	if diff := cmp.Diff(want, box, cmp.AllowUnexported(notdefBox{})); diff != "" {
		t.Errorf("wrong box (-want +got):\n%s", diff)
	}

	// fixed advance, no cap height
	box = makeNotdefBox(1000, 800, 0)
	want = notdefBox{
		advance: 800,
		outer:   funit.Rect16{LLx: 150, LLy: 0, URx: 650, URy: 700},
		inner:   funit.Rect16{LLx: 200, LLy: 50, URx: 600, URy: 650},
	}
	if diff := cmp.Diff(want, box, cmp.AllowUnexported(notdefBox{})); diff != "" {
		t.Errorf("wrong box (-want +got):\n%s", diff)
	}
}

// TestNotdefGenerated checks that glyph 0 of every generated Quire font is
// the .notdef box.
func TestNotdefGenerated(t *testing.T) {
	var files []string
	for _, family := range quireFamilies {
		for _, style := range quireStyles {
			files = append(files, "quire/Quire"+family+"-"+style+".ttf.gz")
		}
	}
	for _, spec := range symbolSpecs {
		files = append(files, "quire/"+spec.fontName+".otf.gz")
	}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			f := readFont(t, file)

			var advance float64
			if f.IsFixedPitch() {
				advance = f.GlyphWidth(0)
			}
			box := makeNotdefBox(float64(f.UnitsPerEm), advance, float64(f.CapHeight))
			if got := f.GlyphWidth(0); got != box.advance {
				t.Errorf("advance %g, want %g", got, box.advance)
			}
			o := box.outer
			wantBBox := rect.Rect{
				LLx: float64(o.LLx), LLy: float64(o.LLy),
				URx: float64(o.URx), URy: float64(o.URy),
			}
			if got := f.GlyphBBox(0); got != wantBBox {
				t.Errorf("bbox %v, want %v", got, wantBBox)
			}

			// regenerating glyph 0 must leave it unchanged
			switch outl := f.Outlines.(type) {
			case *glyf.Outlines:
				old := *outl.Glyphs[0]
				if err := setNotdefGlyf(f); err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(&old, outl.Glyphs[0]); diff != "" {
					t.Errorf("wrong glyph (-want +got):\n%s", diff)
				}
			case *cff.Outlines:
				old := outl.Glyphs[0]
				setNotdefCFF(outl, float64(f.UnitsPerEm), float64(f.CapHeight))
				if diff := cmp.Diff(old, outl.Glyphs[0]); diff != "" {
					t.Errorf("wrong glyph (-want +got):\n%s", diff)
				}
			default:
				t.Fatalf("unexpected outline type %T", f.Outlines)
			}
		})
	}
}

func TestSetNotdefGlyfWrongOutlines(t *testing.T) {
	f := readFont(t, "quire/QuireSymbol.otf.gz")
	if err := setNotdefGlyf(f); err == nil {
		t.Error("CFF outlines accepted")
	}
}
