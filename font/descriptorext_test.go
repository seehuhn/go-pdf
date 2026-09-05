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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/postscript/cid"
	pstype1 "seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	cfffont "seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/squarefont"
)

// TestDescriptorIndependentOfSubset checks that the font descriptor describes
// the font as designed: embedding a single glyph must yield the same entries
// as embedding the whole alphabet.  Only FontName, IsSymbolic and MissingWidth
// may differ, since those describe how the document uses the font.
func TestDescriptorIndependentOfSubset(t *testing.T) {
	cff := func() *sfnt.Font { return makefont.OpenType() }
	ttf := func() *sfnt.Font { return makefont.TrueType() }

	cases := []struct {
		name string
		make func() (font.Layouter, error)
	}{
		{"cff.NewSimple", func() (font.Layouter, error) { return cfffont.NewSimple(cff(), nil) }},
		{"cff.NewComposite", func() (font.Layouter, error) { return cfffont.NewComposite(cff(), nil) }},
		{"truetype.NewSimple", func() (font.Layouter, error) { return truetype.NewSimple(ttf(), nil) }},
		{"truetype.NewComposite", func() (font.Layouter, error) { return truetype.NewComposite(ttf(), nil) }},
		{"opentype.NewSimple/cff", func() (font.Layouter, error) { return opentype.NewSimple(cff(), nil) }},
		{"opentype.NewComposite/cff", func() (font.Layouter, error) { return opentype.NewComposite(cff(), nil) }},
		{"opentype.NewSimple/glyf", func() (font.Layouter, error) { return opentype.NewSimple(ttf(), nil) }},
		{"opentype.NewComposite/glyf", func() (font.Layouter, error) { return opentype.NewComposite(ttf(), nil) }},
	}

	const all = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			few := embedText(t, c.make, ".")
			many := embedText(t, c.make, all)

			for _, fd := range []*font.Descriptor{few, many} {
				fd.FontName = ""
				fd.IsSymbolic = false
				fd.MissingWidth = 0
			}
			if diff := cmp.Diff(many, few); diff != "" {
				t.Errorf("descriptor depends on the subset (-all glyphs +one glyph):\n%s", diff)
			}
		})
	}
}

// embedText embeds a font showing the given text and returns the descriptor
// written to the file.
func embedText(t *testing.T, make func() (font.Layouter, error), text string) *font.Descriptor {
	t.Helper()

	F, err := make()
	if err != nil {
		t.Fatal(err)
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range F.Layout(nil, 10, text).Seq {
		if _, ok := F.Encode(g.GID, g.Text); !ok {
			t.Fatalf("no code allocated for %q", g.Text)
		}
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	obj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	switch d := obj.(type) {
	case *dict.Type1:
		return d.Descriptor
	case *dict.TrueType:
		return d.Descriptor
	case *dict.CIDFontType0:
		return d.Descriptor
	case *dict.CIDFontType2:
		return d.Descriptor
	default:
		t.Fatalf("unexpected font dictionary %T", obj)
		return nil
	}
}

// TestDescriptorStemsFromOriginalFD checks that the stem widths of a CID-keyed
// CFF font are read from the original font DICT 0.  Subsetting renumbers the
// font DICTs, so reading them from the subset would give the values of
// whichever DICT serves the first glyph kept.
func TestDescriptorStemsFromOriginalFD(t *testing.T) {
	const (
		stemFD0 = 100
		stemFD1 = 250
	)

	makeFont := func() *sfnt.Font {
		info := makefont.OpenType()
		outlines := info.Outlines.(*cff.Outlines).Clone()
		outlines.Private = []*pstype1.PrivateDict{
			{StdVW: stemFD0, StdHW: stemFD0 / 2},
			{StdVW: stemFD1, StdHW: stemFD1 / 2},
		}
		// ".notdef" is served by FD 1, so it is FD 1 which the subsetter
		// renumbers to index 0
		outlines.FDSelect = func(gid glyph.ID) int {
			if gid == 0 {
				return 1
			}
			return 0
		}
		// a CID-keyed font carries one matrix per font DICT
		outlines.FontMatrices = []matrix.Matrix{matrix.Identity, matrix.Identity}
		outlines.ROS = &cid.SystemInfo{Registry: "seehuhn.de", Ordering: "test"}
		outlines.GIDToCID = make([]cid.CID, len(outlines.Glyphs))
		for i := range outlines.GIDToCID {
			outlines.GIDToCID[i] = cid.CID(i)
		}
		info.Outlines = outlines
		return info
	}

	want := math.Round(stemFD0 * makeFont().FontMatrix[0] * 1000)

	cases := []struct {
		name string
		make func() (font.Layouter, error)
	}{
		{"cff.NewComposite", func() (font.Layouter, error) { return cfffont.NewComposite(makeFont(), nil) }},
		{"opentype.NewComposite", func() (font.Layouter, error) { return opentype.NewComposite(makeFont(), nil) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fd := embedText(t, c.make, "A")
			if fd.StemV != want {
				t.Errorf("StemV = %g, want %g", fd.StemV, want)
			}
		})
	}
}

// TestDescriptorAcrossTechnologies checks that one design yields the same
// descriptor whichever font technology carries it, and whatever the units of
// the glyph outlines: the descriptor is written in PDF glyph space
// throughout.  Entries a technology has no source for are skipped: TrueType
// fonts record no stem widths, Type 1 fonts no leading or stretch.  Type 3
// descriptors use the font's own glyph space and are checked separately.
func TestDescriptorAcrossTechnologies(t *testing.T) {
	want := font.Descriptor{
		FontFamily:  "SquareFont",
		FontStretch: os2.WidthNormal,
		FontWeight:  os2.WeightMedium,
		FontBBox: rect.Rect{
			LLx: squarefont.SquareLeft,
			LLy: squarefont.SquareBottom,
			URx: squarefont.SquareRight,
			URy: squarefont.SquareTop,
		},
		Ascent:       squarefont.Ascent,
		Descent:      squarefont.Descent,
		Leading:      squarefont.Leading,
		CapHeight:    squarefont.CapHeight,
		XHeight:      squarefont.XHeight,
		StemV:        squarefont.StemWidth,
		StemH:        squarefont.StemWidth,
		MissingWidth: squarefont.NotdefWidth,
	}

	for _, s := range squarefont.All {
		if strings.HasPrefix(s.Label, "Type3") {
			continue
		}
		t.Run(s.Label, func(t *testing.T) {
			expected := want
			switch {
			case strings.HasPrefix(s.Label, "TrueType"):
				expected.StemV, expected.StemH = 0, 0
			case strings.HasPrefix(s.Label, "Type1"):
				expected.Leading = 0
				expected.FontStretch = 0
			}

			got := *embedText(t, func() (font.Layouter, error) { return s.MakeFont(), nil }, "A")
			got.FontName = ""
			got.IsSymbolic = false
			if diff := cmp.Diff(expected, got); diff != "" {
				t.Errorf("descriptor differs (-want +got):\n%s", diff)
			}
		})
	}
}
