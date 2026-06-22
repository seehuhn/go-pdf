// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package builder_test

import (
	"bytes"
	"io"
	"math"
	"math/rand"
	"strings"
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/fonttypes"
	"seehuhn.de/go/pdf/internal/squarefont"
	"seehuhn.de/go/pdf/reader"
)

func TestGetQuadPointsSimple(t *testing.T) {
	F := squarefont.All[0].MakeFont()

	b := builder.New(content.Page, nil, pdf.V2_0)

	b.TextBegin()
	b.TextSetFont(F, 10)
	b.TextFirstLine(100, 100)
	gg := b.TextLayout(nil, "A")
	corners := b.TextGetQuadPoints(gg, 0)
	b.TextShowGlyphs(gg)
	b.TextEnd()

	if b.Err != nil {
		t.Fatal(b.Err)
	}

	expected := []vec.Vec2{
		{X: 101, Y: 98},  // bottom-left
		{X: 105, Y: 98},  // bottom-right
		{X: 105, Y: 108}, // top-right
		{X: 101, Y: 108}, // top-left
	}
	if len(corners) != len(expected) {
		t.Fatalf("expected %d points, got %d", len(expected), len(corners))
	}
	for i := range expected {
		if math.Abs(corners[i].X-expected[i].X) > 1e-6 || math.Abs(corners[i].Y-expected[i].Y) > 1e-6 {
			t.Errorf("point %d: expected (%.6f, %.6f), got (%.6f, %.6f)", i, expected[i].X, expected[i].Y, corners[i].X, corners[i].Y)
		}
	}
}

func TestTextGetQuadPointsComprehensive(t *testing.T) {
	// Expected quad points are derived by hand from the squarefont geometry
	// (see internal/squarefont): glyph "A" has bounding box [0.1, 0.5] × [0.2,
	// 0.6] em and advance 0.5 em; "space" advances 0.25 em with no ink; the
	// font ascent is 0.8 em and descent -0.2 em.  For "A A" at size 10 the
	// text-space box therefore spans x ∈ [1, 12.5] (LLx of the first A to URx
	// of the second, which starts 0.5+0.25 = 0.75 em past the origin) and
	// y ∈ [-2, 8] (descent and ascent dominate the glyph box).  Text rise
	// lifts the top of the glyph box to URy*size + rise.
	testCases := []struct {
		name      string
		fontSize  float64
		setupFunc func(*builder.Builder) *font.GlyphSeq
		expected  []vec.Vec2
	}{
		{
			name:     "identity_transform",
			fontSize: 10.0,
			setupFunc: func(b *builder.Builder) *font.GlyphSeq {
				return b.TextLayout(nil, "A A")
			},
			expected: []vec.Vec2{
				{X: 1, Y: -2}, {X: 12.5, Y: -2}, {X: 12.5, Y: 8}, {X: 1, Y: 8},
			},
		},
		{
			name:     "text_matrix_translate",
			fontSize: 10.0,
			setupFunc: func(b *builder.Builder) *font.GlyphSeq {
				b.TextSetMatrix(matrix.Translate(20, 30))
				return b.TextLayout(nil, "A A")
			},
			// box translated by (20, 30)
			expected: []vec.Vec2{
				{X: 21, Y: 28}, {X: 32.5, Y: 28}, {X: 32.5, Y: 38}, {X: 21, Y: 38},
			},
		},
		{
			name:     "text_matrix_scale",
			fontSize: 10.0,
			setupFunc: func(b *builder.Builder) *font.GlyphSeq {
				b.TextSetMatrix(matrix.Scale(1.5, 1.2))
				return b.TextLayout(nil, "A A")
			},
			// box scaled by (1.5, 1.2)
			expected: []vec.Vec2{
				{X: 1.5, Y: -2.4}, {X: 18.75, Y: -2.4}, {X: 18.75, Y: 9.6}, {X: 1.5, Y: 9.6},
			},
		},
		{
			name:     "text_rise",
			fontSize: 10.0,
			setupFunc: func(b *builder.Builder) *font.GlyphSeq {
				b.TextSetRise(5.0)
				return b.TextLayout(nil, "A A")
			},
			// rise 5 lifts the glyph-box top to URy*size + rise = 6 + 5 = 11
			expected: []vec.Vec2{
				{X: 1, Y: -2}, {X: 12.5, Y: -2}, {X: 12.5, Y: 11}, {X: 1, Y: 11},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testFont := squarefont.All[0].MakeFont()

			b := builder.New(content.Page, nil, pdf.V2_0)
			b.TextBegin()
			b.TextSetFont(testFont, tc.fontSize)

			glyphSeq := tc.setupFunc(b)

			result := b.TextGetQuadPoints(glyphSeq, 0)

			if len(result) != len(tc.expected) {
				t.Fatalf("expected %d points, got %d", len(tc.expected), len(result))
			}
			for i := range tc.expected {
				if math.Abs(result[i].X-tc.expected[i].X) > 1e-6 || math.Abs(result[i].Y-tc.expected[i].Y) > 1e-6 {
					t.Errorf("point %d: expected (%.6f, %.6f), got (%.6f, %.6f)", i, tc.expected[i].X, tc.expected[i].Y, result[i].X, result[i].Y)
				}
			}

			b.TextEnd()
		})
	}
}

func TestGetGlyphQuadPointsStateValidation(t *testing.T) {
	// Test that function returns nil when required text state is not set
	testFont := squarefont.All[0].MakeFont()

	b := builder.New(content.Page, nil, pdf.V2_0)

	// Create a glyph sequence without setting up text state
	glyphSeq := testFont.Layout(nil, 12.0, "A")

	// Should return nil because text state is not properly set
	result := b.TextGetQuadPoints(glyphSeq, 0)
	if result != nil {
		t.Errorf("expected nil result when text state not set, got %v", result)
	}
}

func TestGetGlyphQuadPointsTextMatrixTransform(t *testing.T) {
	// Test combined text matrix and CTM transformation
	testFont := squarefont.All[0].MakeFont()

	b := builder.New(content.Page, nil, pdf.V2_0)

	// Set up text state
	b.TextSetFont(testFont, 10.0)

	// Apply CTM transformation before starting text object
	b.Transform(matrix.Scale(2, 2))

	// Start text object and set text matrix
	b.TextBegin()
	b.TextSetMatrix(matrix.Translate(5, 10))

	glyphSeq := b.TextLayout(nil, "A")

	// The function should account for both text matrix and CTM
	result := b.TextGetQuadPoints(glyphSeq, 0)

	// Should get a valid result (not nil)
	if result == nil {
		t.Error("expected valid result with proper text state, got nil")
	}

	// Should have 4 points
	if len(result) != 4 {
		t.Errorf("expected 4 points, got %d", len(result))
	}

	b.TextEnd()
}

func TestGlyphWidths(t *testing.T) {
	data, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	F := font.Must(standard.TimesRoman.New())

	gg0 := F.Layout(nil, 50, "AB")
	if len(gg0.Seq) != 2 {
		t.Fatal("wrong number of glyphs")
	}

	b := builder.New(content.Page, nil, pdf.V2_0)
	b.TextBegin()
	b.TextSetHorizontalScaling(2)
	b.TextSetFont(F, 50)
	b.TextFirstLine(100, 100)
	gg := &font.GlyphSeq{
		Seq: []font.Glyph{
			{
				GID:     gg0.Seq[0].GID,
				Advance: 100,
				Text:    "A",
			},
			{
				GID:  gg0.Seq[1].GID,
				Text: "B",
			},
		},
	}
	b.TextShowGlyphs(gg)
	b.TextEnd()

	if b.Err != nil {
		t.Fatal(b.Err)
	}

	buf := &bytes.Buffer{}
	if err := writeOps(buf, b.Stream); err != nil {
		t.Fatal(err)
	}

	in := reader.New(pdf.NewExtractor(data))
	var xxOut []float64
	in.Character = func(c font.Code) error {
		x, _ := in.State.GState.GetTextPositionDevice()
		xxOut = append(xxOut, x)
		return nil
	}
	in.State = content.NewState(content.Page, b.Resources)
	bufData := buf.Bytes()
	open := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bufData)), nil
	}
	if err := in.ProcessIter(content.NewScanner(open).NewIter()); err != nil {
		t.Fatal(err)
	}

	if len(xxOut) != 2 {
		t.Fatal("wrong number of glyphs")
	}
	if math.Abs(xxOut[0]-100) > 0.01 {
		t.Errorf("wrong glyph position: %f != 100", xxOut[0])
	}
	if math.Abs(xxOut[1]-200) > 0.01 {
		t.Errorf("wrong glyph position: %f != 200", xxOut[1])
	}
}

func BenchmarkTextLayout(b *testing.B) {
	for _, info := range fonttypes.All {
		b.Run(info.Label, func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				writeDummyDocument(io.Discard, info.MakeFont)
			}
		})
	}
}

func writeDummyDocument(w io.Writer, makeFont func() font.Layouter) error {
	words1 := strings.Fields(sampleText1)
	words2 := strings.Fields(sampleText2)

	paper := document.A4
	doc, err := document.WriteMultiPage(w, paper, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	F := makeFont()

	const leading = 12.0
	setStyle := func(page *document.Page) {
		page.TextSetFont(F, 10)
		page.TextSetLeading(leading)
		page.SetFillColor(color.Black)
	}

	page := doc.AddPage()
	setStyle(page)

	spaceWidth := page.TextLayout(nil, " ").TotalWidth()

	page.TextBegin()
	yPos := paper.URy - 72
	page.TextFirstLine(72, yPos)
	width := paper.Dx() - 2*72

	gg := &font.GlyphSeq{}

	showLine := func(line string) error {
		if yPos < 72 {
			page.TextEnd()
			err = page.Close()
			if err != nil {
				return err
			}
			page = doc.AddPage()
			setStyle(page)
			page.TextBegin()
			yPos = paper.URy - 72
			page.TextFirstLine(72, yPos)
		}
		page.TextShow(line)
		page.TextNextLine()
		yPos -= leading
		return nil
	}

	rng := rand.New(rand.NewSource(0))

	var par []string
	for range 100 {
		n := rng.Intn(9) + 1
		par = par[:0]
		for range n {
			if rng.Intn(2) == 0 {
				par = append(par, words1...)
			} else {
				par = append(par, words2...)
			}
		}

		var line []string
		var lineWidth float64
		for len(par) > 0 {
			var word string
			word, par = par[0], par[1:]
			gg.Reset()
			w := page.TextLayout(gg, word).TotalWidth()
			if len(line) == 0 {
				line = append(line, word)
				lineWidth = w
			} else if lineWidth+w+spaceWidth <= width {
				line = append(line, word)
				lineWidth += w + spaceWidth
			} else {
				err = showLine(strings.Join(line, " "))
				if err != nil {
					return err
				}
				line = line[:0]
				line = append(line, word)
				lineWidth = w
			}
		}
		err = showLine(strings.Join(line, " "))
		if err != nil {
			return err
		}
		if yPos >= 72 {
			showLine("")
		}
	}

	page.TextEnd()
	err = page.Close()
	if err != nil {
		return err
	}

	err = doc.Close()
	if err != nil {
		return err
	}

	return nil
}

// TestTextLayout1 tests that no text content is lost when a glyph sequence
// is laid out.
func TestTextLayout1(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			F, err := gofont.Regular.NewSimple(nil)
			if err != nil {
				t.Fatal(err)
			}
			b := builder.New(content.Page, nil, pdf.V2_0)
			b.TextSetFont(F, 10)

			var testCases = []string{
				"",
				" ",
				"ABC",
				"Hello World",
				"flower", // typeset as ligature
				"fish",   // typeset as ligature
				"ﬂower",  // ligature in source text
				"ﬁsh",    // ligature in source text
			}
			for _, s := range testCases {
				gg := b.TextLayout(nil, s)
				if gg == nil {
					t.Fatal("typesetting failed")
				}
				if gg.Text() != s {
					t.Errorf("wrong text: %s != %s", gg.Text(), s)
				}
			}
		})
	}
}

// TestTextLayout2 tests that ligatures are disabled when character spacing is
// non-zero.
func TestTextLayout2(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			F, err := gofont.Regular.NewSimple(nil)
			if err != nil {
				t.Fatal(err)
			}
			b := builder.New(content.Page, nil, pdf.V2_0)
			b.TextSetFont(F, 10)

			// First make sure the font uses ligatures:
			gg := b.TextLayout(nil, "fi")
			if gg == nil {
				t.Fatal("typesetting failed")
			}
			if len(gg.Seq) != 1 {
				t.Fatal("test is broken")
			}

			// Then make sure that ligatures are disabled when character
			// spacing is non-zero:
			b.TextSetCharacterSpacing(1)
			gg = b.TextLayout(nil, "fi")
			if gg == nil {
				t.Fatal("layout failed")
			}
			if len(gg.Seq) != 2 {
				t.Error("ligatures not disabled")
			}
		})
	}
}

// TestTextLayout3 tests that the width of a glyph sequence scales
// with the font size.
func TestTextLayout3(t *testing.T) {
	F, err := gofont.Regular.NewSimple(nil)
	if err != nil {
		t.Fatal(err)
	}
	b := builder.New(content.Page, nil, pdf.V2_0)

	b.TextSetFont(F, 10)
	L1 := b.TextLayout(nil, "hello world!").TotalWidth()
	b.TextSetFont(F, 20)
	L2 := b.TextLayout(nil, "hello world!").TotalWidth()

	if L1 <= 0 {
		t.Fatalf("invalid width: %f", L1)
	}
	if math.Abs(L2/L1-2) > 0.05 {
		t.Errorf("unexpected width ratio: %f/%f=%f", L2, L1, L2/L1)
	}
}

// Thanks Google Bard, for making up this sentence for me.
// https://g.co/gemini/share/784105073f35
const sampleText1 = "I was weary of sight, weary of acquaintance, weary of familiarity, weary of myself, and weary of all the world; and henceforth all places were alike to me."

// This one is from the actual Moby Dick novel.
const sampleText2 = "With a philosophical flourish Cato throws himself upon his sword; I quietly take to the ship."

// writeOps serialises ops into out via [content.Operator.Format].
func writeOps(out io.Writer, ops []content.Operator) error {
	for _, op := range ops {
		if err := op.Format(out); err != nil {
			return err
		}
	}
	return nil
}
