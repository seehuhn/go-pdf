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
