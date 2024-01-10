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
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	pdfcff "seehuhn.de/go/pdf/font/cff"
)

// The tests in this file check that ghostscripts idea of PDF coincides with
// our own.

// TestLineWidth checks that a vertical line of width 6 colours the correct
// pixels.
func TestLineWidth(t *testing.T) {
	if !haveGhostScript() {
		t.Skip("ghostscript not found")
	}
	img := gsRender(t, 20, 5, pdf.V1_7, func(r *document.Page) error {
		r.SetLineWidth(6.0)
		r.MoveTo(10, 0)
		r.LineTo(10, 5)
		r.Stroke()
		return nil
	})

	rect := img.Bounds()
	for i := rect.Min.X; i < rect.Max.X; i++ {
		for j := rect.Min.Y; j < rect.Max.Y; j++ {
			r, g, b, a := img.At(i, j).RGBA()
			if i >= 4*7 && i < 4*13 {
				// should be black
				if r != 0 || g != 0 || b != 0 || a != 0xffff {
					t.Errorf("pixel (%d,%d) should be black, but is %d,%d,%d,%d", i, j, r, g, b, a)
				}
			} else {
				// should be white
				if r != 0xffff || g != 0xffff || b != 0xffff || a != 0xffff {
					t.Errorf("pixel (%d,%d) should be white, but is %d,%d,%d,%d", i, j, r, g, b, a)
				}
			}
		}
	}
}

// TestTextPositions checks that text positions are correcly updated
// in the graphics state.
func TestTextPositions(t *testing.T) {
	// make a test font
	F := &cff.Font{
		FontInfo: &type1.FontInfo{
			FontName:   "Test",
			Version:    "1.000",
			FontMatrix: []float64{0.0005, 0, 0, 0.0005, 0, 0},
		},
		Outlines: &cff.Outlines{
			Private: []*type1.PrivateDict{
				{BlueValues: []funit.Int16{0, 0}},
			},
			FDSelect: func(glyph.ID) int { return 0 },
			Encoding: make([]glyph.ID, 256),
		},
	}

	g := &cff.Glyph{
		Name:  ".notdef",
		Width: 2000,
	}
	g.MoveTo(0, 0)
	g.LineTo(2000, 0)
	g.LineTo(2000, 2000)
	g.LineTo(0, 2000)
	F.Glyphs = append(F.Glyphs, g)
	for i, w := range []funit.Int16{100, 200, 400, 800} { // 5px, 10px, 20px, 40px
		nameByte := 'A' + byte(i)
		g = &cff.Glyph{
			Name:  string([]byte{nameByte}),
			Width: w,
		}
		g.MoveTo(0, 0)
		g.LineTo(40, 0)
		g.LineTo(40, 2000)
		g.LineTo(0, 2000)
		F.Encoding[nameByte] = glyph.ID(len(F.Glyphs))
		F.Glyphs = append(F.Glyphs, g)
	}

	e := &pdfcff.EmbedInfoCFFSimple{
		Font:       F,
		Encoding:   F.Encoding,
		UnitsPerEm: uint16(math.Round(1 / F.FontInfo.FontMatrix[0])),
		Ascent:     1000,
		CapHeight:  1000,
	}

	testString := pdf.String("CADABX")
	// first print all glyphs in one string
	img1 := gsRender(t, 200, 120, pdf.V1_7, func(r *document.Page) error {
		F, err := embedTestFont(r.Out, e, "F")
		if err != nil {
			return err
		}

		r.TextSetFont(F, 100)
		r.TextStart()
		r.TextFirstLine(10, 10)
		r.TextShowRaw(testString)
		r.TextEnd()

		return nil
	})
	// now print glyphs one-by-one and record the x positions
	var xx []float64
	img2 := gsRender(t, 200, 120, pdf.V1_7, func(r *document.Page) error {
		F, err := embedTestFont(r.Out, e, "F")
		if err != nil {
			return err
		}

		r.TextSetFont(F, 100)
		r.TextStart()
		r.TextFirstLine(10, 10)
		for _, c := range testString {
			xx = append(xx, r.TextMatrix[4])
			r.TextShowRaw(pdf.String{c})
		}
		r.TextEnd()

		return nil
	})
	// finally, print each glyph at the recorded x positions
	img3 := gsRender(t, 200, 120, pdf.V1_7, func(r *document.Page) error {
		F, err := embedTestFont(r.Out, e, "F")
		if err != nil {
			return err
		}

		r.TextSetFont(F, 100)
		for i, c := range testString {
			r.TextStart()
			r.TextFirstLine(xx[i], 10)
			r.TextShowRaw(pdf.String{c})
			r.TextEnd()
		}

		return nil
	})

	// check that all three images are the same
	rect := img1.Bounds()
	if rect != img2.Bounds() || rect != img3.Bounds() {
		t.Errorf("image bounds differ: %v, %v, %v", img1.Bounds(), img2.Bounds(), img3.Bounds())
	}
	for i := rect.Min.X; i < rect.Max.X; i++ {
		for j := rect.Min.Y; j < rect.Max.Y; j++ {
			r1, g1, b1, a1 := img1.At(i, j).RGBA()
			r2, g2, b2, a2 := img2.At(i, j).RGBA()
			r3, g3, b3, a3 := img3.At(i, j).RGBA()
			if r1 != r2 || r1 != r3 || g1 != g2 || g1 != g3 || b1 != b2 || b1 != b3 || a1 != a2 || a1 != a3 {
				t.Errorf("pixel (%d,%d) differs: %d,%d,%d,%d vs %d,%d,%d,%d vs %d,%d,%d,%d", i, j, r1, g1, b1, a1, r2, g2, b2, a2, r3, g3, b3, a3)
			}
		}
	}
}

type testFont struct {
	*pdfcff.EmbedInfoCFFSimple
	ref  pdf.Reference
	name pdf.Name
}

func embedTestFont(w pdf.Putter, e *pdfcff.EmbedInfoCFFSimple, name pdf.Name) (*testFont, error) {
	ref := w.Alloc()
	err := e.Embed(w, ref)
	if err != nil {
		return nil, err
	}
	F := &testFont{
		EmbedInfoCFFSimple: e,
		ref:                ref,
		name:               name,
	}
	return F, nil
}

func (f *testFont) DefaultName() pdf.Name {
	return f.name
}

func (f *testFont) PDFObject() pdf.Object {
	return f.ref
}

func gsRender(t *testing.T, pdfWidth, pdfHeight float64, v pdf.Version, f func(page *document.Page) error) image.Image {
	t.Helper()

	r, err := newGSRenderer(t, pdfWidth, pdfHeight, v)
	if err != nil {
		t.Fatal(err)
	}
	err = f(r.Page)
	if err != nil {
		t.Fatal(err)
	}
	img, err := r.Close()
	if err != nil {
		t.Fatal(err)
	}
	return img
}

type gsRenderer struct {
	Dir     string
	PDFName string

	*document.Page
}

func newGSRenderer(t *testing.T, width, height float64, v pdf.Version) (*gsRenderer, error) {
	t.Helper()

	dir := t.TempDir()

	pdfName := filepath.Join(dir, fmt.Sprintf("test%03d.pdf", gsIndex))
	paper := &pdf.Rectangle{
		URx: width,
		URy: height,
	}
	opt := &pdf.WriterOptions{Version: v}
	doc, err := document.CreateSinglePage(pdfName, paper, opt)
	if err != nil {
		return nil, err
	}

	res := &gsRenderer{
		Dir:     dir,
		PDFName: pdfName,
		Page:    doc,
	}

	return res, nil
}

func (r *gsRenderer) Close() (image.Image, error) {
	err := r.Page.Close()
	if err != nil {
		return nil, err
	}

	pngName := filepath.Join(r.Dir, fmt.Sprintf("test%03d.png", gsIndex))
	gsIndex++

	cmd := exec.Command(
		"gs", "-q",
		"-sDEVICE=png16m", fmt.Sprintf("-r%d", gsResolution),
		"-dTextAlphaBits=4", "-dGraphicsAlphaBits=4",
		"-o", pngName,
		r.PDFName)
	cmd.Dir = r.Dir
	cmd.Stdin = nil
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if len(out) > 0 {
		fmt.Println("unexpected ghostscript output:")
		fmt.Println(string(out))
	}

	fd, err := os.Open(pngName)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	img, err := png.Decode(fd)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func haveGhostScript() bool {
	gsScriptOnce.Do(func() {
		out, err := exec.Command("gs", "-h").Output()
		if err != nil {
			gsScriptFound = false
			return
		}
		gsScriptFound = gsScriptPNGRe.Match(out)
	})
	return gsScriptFound
}

var (
	gsScriptOnce  sync.Once
	gsScriptPNGRe = regexp.MustCompile(`\bpng16m\b`)
	gsScriptFound bool
	gsIndex       int
)

const gsResolution = 4 * 72
