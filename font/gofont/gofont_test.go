// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package gofont

import (
	"testing"

	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/truetype"
)

// TestLigatures checks that letters are correctly combined into ligatures.
func TestLigatures(t *testing.T) {
	ligatures := []string{"ﬀ=ff", "ﬁ=fi", "ﬂ=fl", "ﬃ=ffi", "ﬄ=ffl"}
	for i, Fx := range []Font{Bold, BoldItalic, Italic,
		Medium, MediumItalic, Regular, Smallcaps, SmallcapsItalic,
		Mono, MonoBold, MonoBoldItalic, MonoItalic} {

		F, err := Fx.NewComposite(&truetype.OptionsComposite{CMap: cmap.IdentityH})
		if err != nil {
			t.Fatal(err)
		}

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
				if gg.Seq[0].Text != string(rr[0]) {
					t.Errorf("font %d: ligature %q: unexpected glyph for %q[0]: %q",
						i, lig, lig, gg.Seq[0].Text)
				}

				if gg.Seq[1].Text != "=" {
					// test is broken
					t.Fatalf("font %d: ligature %q: unexpected glyph for %q[1]: %q",
						i, lig, lig, gg.Seq[1].Text)
				}

				if gg.Seq[2].Text != string(rr[2:]) {
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
