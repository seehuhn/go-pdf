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

package sfntglyphs

import (
	"bytes"
	"io"
	"testing"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/parser"

	"seehuhn.de/go/pdf/font/glyphdata"
)

// makeCFF2Font builds a minimal, non-variable CFF2 sfnt font, mirroring
// go-sfnt's cff2_test.go makeCFF2Font helper.
func makeCFF2Font() *sfnt.Font {
	b := func(v float64) cff.Blend { return cff.Blend{Default: v} }
	box := &cff.GlyphCFF2{Cmds: []cff.GlyphOpCFF2{
		{Op: cff.OpMoveTo, Args: []cff.Blend{b(0), b(0)}},
		{Op: cff.OpLineTo, Args: []cff.Blend{b(500), b(0)}},
		{Op: cff.OpLineTo, Args: []cff.Blend{b(500), b(700)}},
		{Op: cff.OpLineTo, Args: []cff.Blend{b(0), b(700)}},
	}}
	o := &cff.OutlinesCFF2{
		Glyphs:   []*cff.GlyphCFF2{box},
		Widths:   []float64{600},
		Private:  []*cff.PrivateCFF2{{}},
		FDSelect: func(glyph.ID) int { return 0 },
	}
	return &sfnt.Font{
		FamilyName: "SfntglyphsCFF2Test",
		UnitsPerEm: 1000,
		Ascent:     700,
		Descent:    -300,
		CapHeight:  700,
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
		Outlines:   o,
	}
}

// FromStream accepts a CFF2-bearing OpenType stream: sfnt.Read no longer
// rejects CFF2 (go-sfnt C6), so the outlines it returns are CFF2.
func TestFromStreamCFF2(t *testing.T) {
	f := makeCFF2Font()
	var buf bytes.Buffer
	if _, err := f.Write(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}

	stream := &glyphdata.Stream{
		Type: glyphdata.OpenTypeCFF,
		WriteTo: func(w io.Writer, _ *glyphdata.Lengths) error {
			_, err := w.Write(buf.Bytes())
			return err
		},
	}

	got, err := FromStream(stream)
	if err != nil {
		t.Fatalf("FromStream: %v", err)
	}
	if !got.IsCFF2() {
		t.Errorf("FromStream returned outlines of type %T, want CFF2", got.Outlines)
	}
}

// TestFromStreamCFF2RoundTrip is a sanity check that the CFF2 fixture parses
// with sfnt.Read directly, independent of the glyphdata.Stream wrapping.
func TestFromStreamCFF2RoundTrip(t *testing.T) {
	f := makeCFF2Font()
	var buf bytes.Buffer
	if _, err := f.Write(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := sfnt.Read(bytes.NewReader(buf.Bytes()), parser.NewBudget(int64(buf.Len())))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !got.IsCFF2() {
		t.Fatal("round-tripped font is not CFF2")
	}
}

// ToStream never panics for a CFF2 font, regardless of the requested tp: a
// CFF2 font reaching ToStream is a caller bug (embedding always instances a
// variable font first, per the vfinstance package), but the failure is
// reported uniformly as a clean error from the deferred WriteTo call, not a
// panic during ToStream itself.
func TestToStreamCFF2ProducesCleanError(t *testing.T) {
	f := makeCFF2Font()

	for _, tp := range []glyphdata.Type{
		glyphdata.TrueType,
		glyphdata.OpenTypeGlyf,
		glyphdata.OpenTypeCFF,
		glyphdata.OpenTypeCFFSimple,
	} {
		t.Run(tp.String(), func(t *testing.T) {
			stream := ToStream(f, tp) // must not panic
			err := stream.WriteTo(io.Discard, nil)
			if err == nil {
				t.Error("WriteTo accepted CFF2 outlines")
			}
		})
	}
}
