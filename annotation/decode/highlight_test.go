// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

// TestDecodeHighlight checks that the /H entry of link and widget
// annotations is decoded correctly: a missing entry yields the default
// mode, and the deprecated "T" (toggle) mode is normalised to "P".
func TestDecodeHighlight(t *testing.T) {
	tests := []struct {
		name string
		h    pdf.Object
		want annotation.Highlight
	}{
		{"missing", nil, annotation.HighlightInvert},
		{"none", pdf.Name("N"), annotation.HighlightNone},
		{"toggle", pdf.Name("T"), annotation.HighlightPush},
		{"unknown", pdf.Name("X"), annotation.Highlight("X")},
	}
	for _, subtype := range []pdf.Name{"Link", "Widget"} {
		for _, tc := range tests {
			t.Run(string(subtype)+"-"+tc.name, func(t *testing.T) {
				dict := pdf.Dict{
					"Subtype": subtype,
					"Rect":    &pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 30},
				}
				if tc.h != nil {
					dict["H"] = tc.h
				}

				x := pdf.NewExtractor(mock.Getter)
				a, err := Annotation(pdf.CursorAt(x, nil), dict, false)
				if err != nil {
					t.Fatal(err)
				}

				var got annotation.Highlight
				switch a := a.(type) {
				case *annotation.Link:
					got = a.Highlight
				case *annotation.Widget:
					got = a.Highlight
				default:
					t.Fatalf("unexpected annotation type %T", a)
				}
				if got != tc.want {
					t.Errorf("wrong highlighting mode: got %q, want %q", got, tc.want)
				}
			})
		}
	}
}
