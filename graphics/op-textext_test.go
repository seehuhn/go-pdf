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

package graphics_test

import (
	"fmt"
	"io"
	"math"
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/testcases"
	"seehuhn.de/go/pdf/internal/dummyfont"
	"seehuhn.de/go/pdf/internal/fonttypes"
	"seehuhn.de/go/pdf/internal/ghostscript"
)

func TestTextPos(t *testing.T) {
	for i, setup := range testcases.All {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			page, err := document.WriteSinglePage(io.Discard, testcases.Paper, pdf.V1_7, nil)
			if err != nil {
				t.Fatal(err)
			}

			page.TextBegin()
			err = setup(page)
			if err != nil {
				t.Fatal(err)
			}
			x, y := page.GetTextPositionDevice()

			expected := testcases.AllGhostscript[i]
			if math.Abs(x-expected.X) > 1 || math.Abs(y-expected.Y) > 1 {
				t.Fatalf("expected x=%f, y=%f, got x=%f, y=%f", expected.X, expected.Y, x, y)
			}
		})
	}
}

// TestTextShowRaw checks that text positions are correcly updated
// in the graphics state.
func TestTextShowRaw(t *testing.T) {
	testString := pdf.String("CADABX")

	type testCase struct {
		fontSize float64
		M        matrix.Matrix
		stretch  float64
	}
	cases := []testCase{
		{100, matrix.Identity, 1},
		// {50, matrix.Scale(2, 2), 1}, // TODO(voss): make this work
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			// first print all glyphs in one string
			img1 := ghostscript.Render(t, 200, 120, pdf.V1_7, func(r *document.Page) error {
				F := dummyfont.Must()

				r.TextBegin()
				r.TextSetFont(F, c.fontSize)
				r.TextSetMatrix(c.M)
				r.TextSetHorizontalScaling(c.stretch)
				r.TextFirstLine(10, 10)
				r.TextShowRaw(testString)
				r.TextEnd()

				return nil
			})
			// now print glyphs one-by-one and record the x positions
			var xx []float64
			img2 := ghostscript.Render(t, 200, 120, pdf.V1_7, func(r *document.Page) error {
				F := dummyfont.Must()

				r.TextBegin()
				r.TextSetFont(F, c.fontSize)
				r.TextSetMatrix(c.M)
				r.TextSetHorizontalScaling(c.stretch)
				r.TextFirstLine(10, 10)
				for i := range testString {
					xx = append(xx, r.TextMatrix[4])
					r.TextShowRaw(testString[i : i+1])
				}
				r.TextEnd()

				return nil
			})
			// finally, print each glyph at the recorded x positions
			img3 := ghostscript.Render(t, 200, 120, pdf.V1_7, func(r *document.Page) error {
				F := dummyfont.Must()

				r.TextSetFont(F, 100)
				for i := range testString {
					r.TextBegin()
					r.TextSetFont(F, c.fontSize)
					r.TextSetMatrix(c.M)
					r.TextSetHorizontalScaling(c.stretch)
					r.TextFirstLine(xx[i], 10)
					r.TextShowRaw(testString[i : i+1])
					r.TextEnd()
				}

				return nil
			})

			// check that all three images are the same
			rect := img1.Bounds()
			if rect != img2.Bounds() || rect != img3.Bounds() {
				t.Errorf("image bounds differ: %v, %v, %v", img1.Bounds(), img2.Bounds(), img3.Bounds())
			}
			count := 0
		pixLoop:
			for i := rect.Min.X; i < rect.Max.X; i++ {
				for j := rect.Min.Y; j < rect.Max.Y; j++ {
					r1, g1, b1, a1 := img1.At(i, j).RGBA()
					r2, g2, b2, a2 := img2.At(i, j).RGBA()
					r3, g3, b3, a3 := img3.At(i, j).RGBA()
					if r1 != r2 || r1 != r3 ||
						g1 != g2 || g1 != g3 ||
						b1 != b2 || b1 != b3 ||
						a1 != a2 || a1 != a3 {
						t.Errorf("pixel (%d,%d) differs: %d vs %d vs %d",
							i, j,
							[]uint32{r1, g1, b1, a1},
							[]uint32{r2, g2, b2, a2},
							[]uint32{r3, g3, b3, a3})
						count++
						if count > 10 {
							break pixLoop
						}
					}
				}
			}
		})
	}
}

// TestTextPositions checks that text positions are correcly updated
// in the graphics state.
func TestTextShowRaw2(t *testing.T) {
	testString := ".MiAbc"
	// TODO(voss): also try PDF.V2_0, once
	// https://bugs.ghostscript.com/show_bug.cgi?id=707475 is resolved.
	for _, sample := range fonttypes.All {
		t.Run(sample.Label, func(t *testing.T) {
			const fontSize = 100
			var s pdf.String

			// First print glyphs one-by-one and record the x positions.
			var xx []float64
			img1 := ghostscript.Render(t, 400, 120, pdf.V1_7, func(r *document.Page) error {
				F := sample.MakeFont(r.RM)

				r.TextSetFont(F, fontSize)
				r.TextBegin()
				r.TextFirstLine(10, 10)

				_, E, err := pdf.ResourceManagerEmbed(r.RM, F)
				if err != nil {
					return err
				}

				for _, g := range r.TextLayout(nil, testString).Seq {
					xx = append(xx, r.TextMatrix[4])
					s, _ = E.(font.EmbeddedLayouter).AppendEncoded(s[:0], g.GID, string(g.Text))

					r.TextShowRaw(s)
				}
				r.TextEnd()

				return nil
			})
			// Then print each glyph at the recorded x positions.
			img2 := ghostscript.Render(t, 400, 120, pdf.V1_7, func(r *document.Page) error {
				F := sample.MakeFont(r.RM)
				r.TextSetFont(F, fontSize)

				_, E, err := pdf.ResourceManagerEmbed(r.RM, F)
				if err != nil {
					return err
				}

				for i, g := range r.TextLayout(nil, testString).Seq {
					r.TextBegin()
					r.TextFirstLine(xx[i], 10)
					s, _ = E.(font.EmbeddedLayouter).AppendEncoded(s[:0], g.GID, string(g.Text))
					r.TextShowRaw(s)
					r.TextEnd()
				}

				return nil
			})

			// check that all three images are the same
			rect := img1.Bounds()
			if rect != img2.Bounds() {
				t.Errorf("image bounds differ: %v, %v", img1.Bounds(), img2.Bounds())
			}
			for i := rect.Min.X; i < rect.Max.X; i++ {
				for j := rect.Min.Y; j < rect.Max.Y; j++ {
					r1, g1, b1, a1 := img1.At(i, j).RGBA()
					r2, g2, b2, a2 := img2.At(i, j).RGBA()
					if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
						t.Errorf("pixel (%d,%d) differs: %d vs %d",
							i, j,
							[]uint32{r1, g1, b1, a1},
							[]uint32{r2, g2, b2, a2})
						goto tooMuch
					}
				}
			}
		tooMuch:
		})
	}
}
