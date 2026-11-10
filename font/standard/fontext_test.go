// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package standard_test

import (
	"fmt"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/sfnt/os2"
)

// TestEmbedStandard tests that the 14 standard PDF fonts can be
// used and that the font file is not embedded.
func TestEmbedStandard(t *testing.T) {
	for _, standardFont := range standard.All {
		for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
			t.Run(fmt.Sprintf("%s@%s", standardFont, v), func(t *testing.T) {
				data, _ := memfile.NewPDFWriter(t, v, nil)
				rm := pdf.NewResourceManager(data)

				// Embed the font into a PDF file:

				F := font.Must(standardFont.New())
				ref, err := rm.Embed(F)
				if err != nil {
					t.Fatal(err)
				}

				var testText string
				switch standardFont {
				case standard.Symbol:
					testText = "∀"
				case standard.ZapfDingbats:
					testText = "♠"
				default:
					testText = "Hello World"
				}
				gg := F.Layout(nil, 10, testText)
				for _, g := range gg.Seq { // allocate codes
					// Use the actual glyph width, not the kerned advance
					_, _ = F.Encode(g.GID, g.Text)
				}

				err = rm.Close()
				if err != nil {
					t.Fatal(err)
				}

				// Read back the font dictionary and check that everything is
				// as expected:

				x := pdf.NewExtractor(data)
				dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
				if err != nil {
					t.Fatal(err)
				}

				t1Dict, ok := dictObj.(*dict.Type1)
				if !ok {
					t.Fatalf("unexpected font dictionary type %T", dictObj)
				}

				if t1Dict.PostScriptName != standardFont.PostScriptName() {
					t.Errorf("unexpected PostScriptName %q", t1Dict.PostScriptName)
				}
				if t1Dict.FontFile != nil {
					t.Errorf("unexpected embedded font file, expected external font")
				}
			})
		}
	}
}

// TestWidthsOmitted checks that a non-embedded standard font is written
// without /Widths and /FontDescriptor.
func TestWidthsOmitted(t *testing.T) {
	tests := []struct {
		name string
		font standard.Font
		text string
	}{
		{"plain", standard.Helvetica, "Hello World"},
		{"plain oblique", standard.HelveticaOblique, "Hello World"},
		{"Helvetica mu", standard.Helvetica, "\u00b5m"},
		{"Times-Bold mu", standard.TimesBold, "\u00b5m"},
		{"HelveticaOblique fraction", standard.HelveticaOblique, "1\u20442"},
		{"Symbol", standard.Symbol, "\u03b1\u03b2"},
		{"ZapfDingbats", standard.ZapfDingbats, "\u2701\u2702"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			if !w.GetOptions().HasAny(pdf.OptTrimStandardFonts) {
				t.Fatal("writer does not trim standard fonts")
			}
			rm := pdf.NewResourceManager(w)

			F := font.Must(tc.font.New())
			ref, err := rm.Embed(F)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range F.Layout(nil, 10, tc.text).Seq {
				if _, ok := F.Encode(g.GID, g.Text); !ok {
					t.Fatalf("cannot encode %q", g.Text)
				}
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}

			obj, err := pdf.Resolve(w, ref)
			if err != nil {
				t.Fatal(err)
			}
			fontDict, ok := obj.(pdf.Dict)
			if !ok {
				t.Fatalf("unexpected font dictionary type %T", obj)
			}
			for _, key := range []pdf.Name{"Widths", "FontDescriptor"} {
				if _, present := fontDict[key]; present {
					t.Errorf("unexpected /%s", key)
				}
			}
		})
	}
}

// TestDescriptorWeight checks that the font descriptor of a standard font
// has the bold weight exactly for the bold faces.
func TestDescriptorWeight(t *testing.T) {
	for _, f := range standard.All {
		inst, err := f.New()
		if err != nil {
			t.Fatal(err)
		}
		want := os2.WeightNormal
		if strings.Contains(string(f), "Bold") {
			want = os2.WeightBold
		}
		if got := inst.Descriptor.FontWeight; got != want {
			t.Errorf("%s: weight %d, want %d", f, got, want)
		}
	}
}
