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
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

// AddAppearance must report an error, not panic, when the target PDF version
// is too old for the operators the appearance needs (e.g. `gs`, PDF 1.2+).
// Readers synthesize appearances at the document's version, where the input
// is untrusted, so a malformed low-version file must not crash the caller.
func TestAddAppearanceLowVersion(t *testing.T) {
	rect := pdf.Rectangle{LLx: 0, LLy: 0, URx: 20, URy: 20}

	build := map[string]func() annotation.Annotation{
		"text": func() annotation.Annotation {
			return &annotation.Text{Common: annotation.Common{Rect: rect}}
		},
		"widget": func() annotation.Annotation {
			return combWidget("AB", 6, pdf.TextAlignLeft)
		},
	}

	for name, mk := range build {
		t.Run(name, func(t *testing.T) {
			// pre-1.2: gs is unavailable, so building must fail with an error
			if err := NewStyle(pdf.V1_1).AddAppearance(mk()); err == nil {
				t.Error("expected an error for PDF 1.1, got nil")
			}
			// 1.2 and later: building succeeds
			if err := NewStyle(pdf.V1_2).AddAppearance(mk()); err != nil {
				t.Errorf("unexpected error for PDF 1.2: %v", err)
			}
		})
	}
}
