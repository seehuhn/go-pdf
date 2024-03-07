// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package gofont

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// TestLigatures checks that letters are correctly combined into ligatures.
func TestLigatures(t *testing.T) {
	ligatures := []string{"ﬀ=ff", "ﬁ=fi", "ﬂ=fl", "ﬃ=ffi", "ﬄ=ffl"}
	for i, F := range []font.Font{GoBold, GoBoldItalic, GoItalic,
		GoMedium, GoMediumItalic, GoRegular, GoSmallcaps, GoSmallcapsItalic,
		GoMono, GoMonoBold, GoMonoBoldItalic, GoMonoItalic} {

		data := pdf.NewData(pdf.V2_0)
		E, err := F.Embed(data, nil)
		if err != nil {
			t.Fatal(err)
		}

		geom := E.GetGeometry()

		for _, lig := range ligatures {
			gg := E.Layout(nil, 10, lig)

			rr := []rune(lig)
			if gg.Seq[0].GID == 0 {
				// The ligature is not present in the font.
				continue
			}

			var ligIsUsed bool
			if len(gg.Seq) == 3 {
				// Glyphs have been combined.
				ligIsUsed = true
				if gg.Seq[0].GID != gg.Seq[2].GID {
					t.Errorf("font %d: ligature %q: unexpected GIDs: %d %d %d",
						i, lig, gg.Seq[0].GID, gg.Seq[1].GID, gg.Seq[2].GID)
				}
				if string(gg.Seq[0].Text) != string(rr[0]) {
					t.Errorf("font %d: ligature %q: unexpected glyph for %q[0]: %q",
						i, lig, lig, gg.Seq[0].Text)
				}

				if string(gg.Seq[1].Text) != "=" {
					// test is broken
					t.Fatalf("font %d: ligature %q: unexpected glyph for %q[1]: %q",
						i, lig, lig, gg.Seq[1].Text)
				}

				if string(gg.Seq[2].Text) != string(rr[2:]) {
					t.Errorf("font %d: ligature %q: unexpected glyph for %q[2]: %q",
						i, lig, lig, gg.Seq[2].Text)
				}
			} else {
				// Glyphs have not been combined.
				ligIsUsed = false
			}

			if ligIsUsed != !geom.IsFixedPitch() {
				t.Errorf("font %d: ligature %q: isFixedPitch=%t but ligIsUsed=%t",
					i, lig, geom.IsFixedPitch(), ligIsUsed)
			}
		}
	}
}
