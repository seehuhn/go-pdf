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

package standard

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestGlyphLists tests that the glyph lists of the 14 standard PDF
// fonts are consistent between the .pfb and the .afm files.
func TestGlyphLists(t *testing.T) {
	for _, G := range All {
		F := G.New()
		psFont := F.Font
		metrics := F.Metrics

		glyphNames1 := psFont.GlyphList()
		glyphNames2 := metrics.GlyphList()
		if d := cmp.Diff(glyphNames1, glyphNames2); d != "" {
			t.Errorf("%-22s differ: %s", G, d)
		}
	}
}

// TestGlyphWidths tests that the glyph widths of the 14 standard PDF
// fonts are consistent between the .pfb and the .afm files.
func TestGlyphWidths(t *testing.T) {
	for _, G := range All {
		F := G.New()
		psFont := F.Font
		metrics := F.Metrics

		for name, g := range psFont.Glyphs {
			w1 := g.WidthX
			w2 := metrics.Glyphs[name].WidthX
			if w1 != w2 {
				t.Errorf("%-22s %-8s width=%g, claimedWidth=%g",
					G, name, w1, w2)
			}
		}
	}
}

// TestBlankGlyphs checks that glyphs are marked as blank in the
// metrics file if and only if they are blank in the .pfb file.
func TestBlankGlyphs(t *testing.T) {
	for _, G := range All {
		F := G.New()
		psFont := F.Font
		metrics := F.Metrics
		for name, g := range psFont.Glyphs {
			isBlank := len(g.Cmds) == 0
			claimedBlank := metrics.Glyphs[name].BBox.IsZero()
			// TODO(voss): fix this for the .notdef glyphs by
			// life-patching the type 1 fonts after loading.
			if isBlank != claimedBlank && name != ".notdef" {
				t.Errorf("%-22s %-8s isBlank=%v, claimedBlank=%v",
					G, name, isBlank, claimedBlank)
			}
		}
	}
}

// TestLigatures checks that letters are correctly combined into ligatures.
func TestLigatures(t *testing.T) {
	ligatures := []string{"ﬀ=ff", "ﬁ=fi", "ﬂ=fl", "ﬃ=ffi", "ﬄ=ffl"}
	for i, G := range All {
		F := G.New()

		geom := F.GetGeometry()

		for _, lig := range ligatures {
			gg := F.Layout(nil, 10, lig)

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
