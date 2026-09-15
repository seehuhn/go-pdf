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

package outline

import (
	"math"
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/path"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/internal/debug/makefont"
)

// testFonts returns one font of every outline flavour the package handles.
// All three carry the per-FD matrix as the identity, so composing it is a
// no-op here; TestFDMatrix covers the composition itself.
func testFonts() map[string]*sfnt.Font {
	return map[string]*sfnt.Font{
		"glyf":    makefont.TrueType(),
		"cff":     makefont.OpenType(),
		"cff cid": makefont.OpenTypeCID(),
	}
}

// numSegments counts the segments a path yields.  A nil path would panic
// here, which is the point: the functions must never return one.
func numSegments(p path.Path) int {
	n := 0
	for range p {
		n++
	}
	return n
}

// TestTextSpace checks that an outline is placed in text space, by comparing
// its bounding box with the one the font reports in PDF glyph space.  The two
// are computed along different routes, so this pins the font matrix.
func TestTextSpace(t *testing.T) {
	for name, info := range testFonts() {
		t.Run(name, func(t *testing.T) {
			bboxes := info.GlyphBBoxesPDF()
			for gid := range glyph.ID(info.NumGlyphs()) {
				p := SFNT(info, gid)
				if numSegments(p) == 0 { // blank glyph
					continue
				}
				got, want := p.BBox(), bboxes[gid]
				for _, d := range []float64{
					got.LLx - want.LLx/1000, got.LLy - want.LLy/1000,
					got.URx - want.URx/1000, got.URy - want.URy/1000,
				} {
					if math.Abs(d) > 1e-6 {
						t.Errorf("glyph %d bbox %v, want %v/1000", gid, got, want)
						break
					}
				}
			}
		})
	}
}

// TestCFFMatchesSFNT checks that the two entry points agree where both apply.
// A CFF font can be reached either way, and a caller must not be able to tell
// which route a font backend took.
func TestCFFMatchesSFNT(t *testing.T) {
	for _, name := range []string{"cff", "cff cid"} {
		t.Run(name, func(t *testing.T) {
			info := testFonts()[name]
			cffFont := info.AsCFF()
			if cffFont == nil {
				t.Fatal("no CFF outlines")
			}
			for gid := range glyph.ID(info.NumGlyphs()) {
				viaSFNT, viaCFF := SFNT(info, gid), CFF(cffFont, gid)
				if numSegments(viaCFF) != numSegments(viaSFNT) {
					t.Fatalf("glyph %d: %d segments via CFF, %d via SFNT",
						gid, numSegments(viaCFF), numSegments(viaSFNT))
				}
				if numSegments(viaSFNT) == 0 {
					continue
				}
				if got, want := viaCFF.BBox(), viaSFNT.BBox(); got != want {
					t.Errorf("glyph %d: bbox %v via CFF, %v via SFNT", gid, got, want)
				}
			}
		})
	}
}

// TestFDMatrix checks that the per-FD font matrix of a CID-keyed CFF font is
// composed with the top-level one.  The fonts in testFonts all leave it as
// the identity, where a missing composition cannot be seen, so this scales it
// and checks the outline follows.
func TestFDMatrix(t *testing.T) {
	f := makefont.OpenTypeCID().AsCFF()
	if !f.IsCIDKeyed() || len(f.FontMatrices) != 1 {
		t.Fatalf("test font has %d per-FD matrices, CID-keyed %v",
			len(f.FontMatrices), f.IsCIDKeyed())
	}

	gid := glyph.ID(0)
	for int(gid) < f.NumGlyphs() && len(f.Glyphs[gid].Cmds) == 0 {
		gid++
	}
	if int(gid) >= f.NumGlyphs() {
		t.Fatal("test font has no glyph with an outline")
	}
	before := CFF(f, gid).BBox()

	f.FontMatrices[0] = matrix.Matrix{2, 0, 0, 2, 0, 0}
	after := CFF(f, gid).BBox()

	for _, d := range []float64{
		after.LLx - 2*before.LLx, after.LLy - 2*before.LLy,
		after.URx - 2*before.URx, after.URy - 2*before.URy,
	} {
		if math.Abs(d) > 1e-9 {
			t.Errorf("doubling the per-FD matrix gave bbox %v, want twice %v",
				after, before)
			break
		}
	}
}
