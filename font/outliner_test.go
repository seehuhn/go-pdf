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

package font_test

import (
	"math"
	"sync"
	"testing"
	"time"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/race"
)

// outlinerWriters builds one instance of every font writer which holds its
// glyph program in memory.
func outlinerWriters(t *testing.T) map[string]font.Layouter {
	t.Helper()
	ww := map[string]font.Layouter{}
	must := func(name string, l font.Layouter, err error) {
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ww[name] = l
	}
	l, err := type1.New(makefont.Type1(), makefont.AFM())
	must("type1", l, err)
	s, err := truetype.NewSimple(makefont.TrueType(), nil)
	must("truetype simple", s, err)
	c, err := truetype.NewComposite(makefont.TrueType(), nil)
	must("truetype composite", c, err)
	cs, err := cff.NewSimple(makefont.OpenType(), nil)
	must("cff simple", cs, err)
	cc, err := cff.NewComposite(makefont.OpenTypeCID(), nil)
	must("cff composite", cc, err)
	og, err := opentype.NewSimple(makefont.TrueType(), nil)
	must("opentype simple glyf", og, err)
	oc, err := opentype.NewSimple(makefont.OpenType(), nil)
	must("opentype simple cff", oc, err)
	ogc, err := opentype.NewComposite(makefont.TrueType(), nil)
	must("opentype composite glyf", ogc, err)
	occ, err := opentype.NewComposite(makefont.OpenTypeCID(), nil)
	must("opentype composite cff", occ, err)
	return ww
}

// TestOutliner checks, for every writer, that the glyph a CID selects is the
// glyph the text was laid out with, that its outline is the glyph's own, and
// that a CID allocated after a first lookup is found as well.
func TestOutliner(t *testing.T) {
	for name, l := range outlinerWriters(t) {
		t.Run(name, func(t *testing.T) {
			o, ok := l.(font.Outliner)
			if !ok {
				t.Fatalf("%T does not implement font.Outliner", l)
			}

			// CID 0 always selects the notdef glyph
			if gid, ok := o.GlyphID(0); !ok || gid != 0 {
				t.Errorf("CID 0 selected glyph %d, ok %v; want glyph 0, ok true", gid, ok)
			}
			// a CID beyond the font has no glyph
			if _, ok := o.GlyphID(1<<20 + 3); ok {
				t.Error("GlyphID far beyond the font reports a glyph")
			}

			checkText(t, o, l, "A b")
			// codes allocated after the first lookups are seen too
			checkText(t, o, l, "x y")
		})
	}
}

// checkText lays out and encodes s, then reads every code back through the
// outliner and compares with the layout.
func checkText(t *testing.T, o font.Outliner, l font.Layouter, s string) {
	t.Helper()
	geom := l.GetGeometry()
	codec := l.Codec()

	seq := l.Layout(nil, 1, s)
	var codes pdf.String
	for _, g := range seq.Seq {
		c, ok := l.Encode(g.GID, g.Text)
		if !ok {
			t.Fatalf("cannot encode glyph %d", g.GID)
		}
		codes = codec.AppendCode(codes, c)
	}

	i := 0
	for code := range l.Codes(codes) {
		want := seq.Seq[i].GID
		i++
		gid, ok := o.GlyphID(code.CID)
		if !ok {
			t.Errorf("%q: CID %d has no glyph", s, code.CID)
			continue
		}
		if gid != want {
			t.Errorf("%q: CID %d selects glyph %d, want %d", s, code.CID, gid, want)
		}

		// the outline is always usable, in particular it is never a nil
		// iterator which would panic when ranged over
		p := o.Outline(gid)
		numSeg := 0
		for range p {
			numSeg++
		}
		if geom.IsBlank(gid) {
			if numSeg != 0 {
				t.Errorf("%q: blank glyph %d has %d path segments", s, gid, numSeg)
			}
			continue
		}
		if numSeg == 0 {
			t.Errorf("%q: glyph %d has an empty outline", s, gid)
			continue
		}
		// the outline is the glyph's own: it spans the extent the font
		// reports for the glyph, in text space
		got, ext := p.BBox(), geom.GlyphExtents[gid]
		for _, d := range []float64{got.LLx - ext.LLx, got.LLy - ext.LLy, got.URx - ext.URx, got.URy - ext.URy} {
			if math.Abs(d) > 1e-3 {
				t.Errorf("%q: glyph %d outline bbox %v, want %v", s, gid, got, ext)
				break
			}
		}
	}
	if i != len(seq.Seq) {
		t.Errorf("%q: decoded %d codes, want %d", s, i, len(seq.Seq))
	}
}

// TestOutlinerConcurrentReaders checks the promise [font.Outliner] makes about
// concurrent use: readers may use an instance while codes are still being
// allocated, a CID which has a glyph keeps that glyph, and a glyph's outline
// never changes.
//
// The readers assert only what the interface guarantees.  Which CIDs exist at
// any moment is up to the writer, so a CID without a glyph is skipped rather
// than treated as a failure.
func TestOutlinerConcurrentReaders(t *testing.T) {
	if !race.Enabled {
		// Only the race detector can see the reads and writes overlap, so
		// without it the test would burn CPU without being able to fail.
		t.Skip("needs the race detector; run go test -race")
	}

	for name, l := range outlinerWriters(t) {
		t.Run(name, func(t *testing.T) {
			o, ok := l.(font.Outliner)
			if !ok {
				t.Fatalf("%T does not implement font.Outliner", l)
			}

			done := make(chan struct{})
			go func() {
				defer close(done)
				for _, r := range "The quick brown fox jumps over the lazy dog" {
					seq := l.Layout(nil, 1, string(r))
					for _, g := range seq.Seq {
						l.Encode(g.GID, g.Text)
					}

					// Pace the writer, so that the readers spend their time on
					// an instance which is still growing rather than on one
					// which is already final.
					time.Sleep(50 * time.Microsecond)
				}
			}()

			var wg sync.WaitGroup
			for range 4 {
				wg.Go(func() {

					// what this reader has seen so far; each reader keeps its
					// own record, so the readers share no state
					glyphs := make(map[cid.CID]glyph.ID)
					boxes := make(map[glyph.ID]rect.Rect)

					for {
						for c := range cid.CID(300) {
							gid, ok := o.GlyphID(c)
							if !ok {
								continue
							}
							if prev, seen := glyphs[c]; seen && prev != gid {
								t.Errorf("CID %d selected glyph %d, now %d", c, prev, gid)
							}
							glyphs[c] = gid

							bbox := o.Outline(gid).BBox()
							if prev, seen := boxes[gid]; seen && prev != bbox {
								t.Errorf("outline of glyph %d was %v, now %v", gid, prev, bbox)
							}
							boxes[gid] = bbox
						}

						select {
						case <-done:
							return
						default:
						}
					}
				})
			}
			wg.Wait()
		})
	}
}
