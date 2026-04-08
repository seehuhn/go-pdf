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

package main

import (
	"fmt"
	"io"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/bitmap"
	pdfimg "seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/graphics/jbig2"
	"seehuhn.de/go/pdf/graphics/text"
)

func main() {
	err := run("test.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("wrote test.pdf")
}

func run(filename string) error {
	doc, err := document.CreateMultiPage(filename, document.A4, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	font := standard.Helvetica.New()

	// page 1: generic region encoding (direct bitmap)
	err = genericPage(doc, font)
	if err != nil {
		return err
	}

	// page 2: symbol dictionary + text region
	err = symbolPage(doc, font)
	if err != nil {
		return err
	}

	return doc.Close()
}

// genericPage creates a page showing a generic region JBIG2 image
// next to the same image encoded as a standard 1-bit image mask.
func genericPage(doc *document.MultiPage, font *type1.Instance) error {
	page := doc.AddPage()

	F := text.F{Font: font, Size: 16}
	Fsmall := text.F{Font: font, Size: 10}

	text.Show(page.Builder,
		text.M{X: 72, Y: 780},
		F, "JBIG2 Generic Region Test",
	)
	text.Show(page.Builder,
		text.M{X: 72, Y: 760},
		Fsmall, "Both images should look identical.",
	)

	// create a test pattern: a 64×64 bitmap with concentric rectangles
	bm := bitmap.New(64, 64)
	for ring := 0; ring < 32; ring += 4 {
		for x := ring; x < 64-ring; x++ {
			bm.SetPixel(x, ring, true)
			bm.SetPixel(x, 63-ring, true)
		}
		for y := ring; y < 64-ring; y++ {
			bm.SetPixel(ring, y, true)
			bm.SetPixel(63-ring, y, true)
		}
	}

	// left: JBIG2 encoded
	jbig2Img, err := newJBIG2GenericImage(bm)
	if err != nil {
		return err
	}
	drawLabeledImage(page, Fsmall, "JBIG2Decode", jbig2Img, 100, 500, 200)

	// right: standard image mask (Flate-compressed)
	maskImg := bitmapToMask(bm)
	drawLabeledImage(page, Fsmall, "Image Mask (Flate)", maskImg, 320, 500, 200)

	// second row: a more complex pattern (checkerboard)
	checker := bitmap.New(64, 64)
	for y := range 64 {
		for x := range 64 {
			if (x/4+y/4)%2 == 0 {
				checker.SetPixel(x, y, true)
			}
		}
	}

	jbig2Check, err := newJBIG2GenericImage(checker)
	if err != nil {
		return err
	}
	drawLabeledImage(page, Fsmall, "JBIG2 Checkerboard", jbig2Check, 100, 240, 200)

	maskCheck := bitmapToMask(checker)
	drawLabeledImage(page, Fsmall, "Flate Checkerboard", maskCheck, 320, 240, 200)

	return page.Close()
}

// symbolPage creates a page demonstrating JBIG2 symbol dictionary + text
// region encoding, placing repeated glyphs via a shared dictionary.
func symbolPage(doc *document.MultiPage, font *type1.Instance) error {
	page := doc.AddPage()

	F := text.F{Font: font, Size: 16}
	Fsmall := text.F{Font: font, Size: 10}

	text.Show(page.Builder,
		text.M{X: 72, Y: 780},
		F, "JBIG2 Symbol Dictionary Test",
	)
	text.Show(page.Builder,
		text.M{X: 72, Y: 760},
		Fsmall, "Both images should look identical.",
	)

	// define 3 small symbol bitmaps (simple geometric shapes)
	symSquare := bitmap.New(8, 8)
	for y := range 8 {
		for x := range 8 {
			if x == 0 || x == 7 || y == 0 || y == 7 {
				symSquare.SetPixel(x, y, true)
			}
		}
	}

	symCross := bitmap.New(8, 8)
	for i := range 8 {
		symCross.SetPixel(3, i, true)
		symCross.SetPixel(4, i, true)
		symCross.SetPixel(i, 3, true)
		symCross.SetPixel(i, 4, true)
	}

	symDiamond := bitmap.New(8, 8)
	for i := range 4 {
		symDiamond.SetPixel(3-i, i, true)
		symDiamond.SetPixel(4+i, i, true)
		symDiamond.SetPixel(3-i, 7-i, true)
		symDiamond.SetPixel(4+i, 7-i, true)
	}

	symbols := []*bitmap.Bitmap{symSquare, symCross, symDiamond}

	// create placement pattern: symbols in a grid
	const regionW, regionH = 80, 40
	var placements []placement
	symIdx := 0
	for row := range 4 {
		for col := range 8 {
			placements = append(placements, placement{
				symID: symIdx % 3,
				x:     col * 10,
				y:     (row+1)*8 - 1, // BOTTOMLEFT: y is the bottom
			})
			symIdx++
		}
	}

	// left: JBIG2 symbol dictionary + text region
	jbig2Sym, err := newJBIG2SymbolImage(symbols, placements, regionW, regionH)
	if err != nil {
		return err
	}
	drawLabeledImage(page, Fsmall, "JBIG2 Symbol Dict", jbig2Sym, 100, 480, 240)

	// right: reference image (render the same placements manually into a bitmap)
	refBm := bitmap.New(regionW, regionH)
	for _, p := range placements {
		sym := symbols[p.symID]
		// BOTTOMLEFT: bottom-left at (x, y), so top-left at (x, y-h+1)
		topY := p.y - sym.Height() + 1
		refBm.Combine(sym, p.x, topY, bitmap.CombOpOR)
	}
	refImg := bitmapToMask(refBm)
	drawLabeledImage(page, Fsmall, "Reference (Flate)", refImg, 100, 220, 240)

	return page.Close()
}

// drawLabeledImage draws an XObject at (x, y) scaled to size, with a label.
func drawLabeledImage(
	page *document.Page,
	f text.F,
	label string,
	img graphics.XObject,
	x, y, size float64,
) {
	text.Show(page.Builder, text.M{X: x, Y: y + size + 8}, f, label)
	page.PushGraphicsState()
	page.Transform(matrix.Translate(x, y))
	page.Transform(matrix.Scale(size, size))
	page.DrawXObject(img)
	page.PopGraphicsState()
}

// newJBIG2GenericImage creates a JBIG2 XObject using generic region encoding.
func newJBIG2GenericImage(bm *bitmap.Bitmap) (*jbig2Image, error) {
	enc := jbig2.NewEncoder()
	pg := jbig2.NewPage(bm.Width(), bm.Height())
	pg.AddGenericRegion(bm, 0, 0, nil)

	pageData, err := enc.EncodePage(pg)
	if err != nil {
		return nil, err
	}

	return &jbig2Image{
		width:    bm.Width(),
		height:   bm.Height(),
		pageData: pageData,
	}, nil
}

// newJBIG2SymbolImage creates a JBIG2 XObject with a symbol dictionary
// and text region.
func newJBIG2SymbolImage(
	symbols []*bitmap.Bitmap,
	pp []placement,
	width, height int,
) (*jbig2Image, error) {
	enc := jbig2.NewEncoder()
	for _, sym := range symbols {
		enc.AddSymbol(sym)
	}

	globals, err := enc.Globals()
	if err != nil {
		return nil, err
	}

	instances := make([]jbig2.Instance, len(pp))
	for i, p := range pp {
		instances[i] = jbig2.Instance{
			SymbolID: p.symID,
			X:        p.x,
			Y:        p.y,
		}
	}

	pg := jbig2.NewPage(width, height)
	pg.AddTextRegion(&jbig2.TextRegion{
		Width:     width,
		Height:    height,
		Instances: instances,
	})

	pageData, err := enc.EncodePage(pg)
	if err != nil {
		return nil, err
	}

	return &jbig2Image{
		width:    width,
		height:   height,
		globals:  globals,
		pageData: pageData,
	}, nil
}

// jbig2Image embeds JBIG2-encoded data as a PDF image XObject.
type jbig2Image struct {
	width, height int
	globals       []byte // JBIG2Globals stream (nil if none)
	pageData      []byte // JBIG2 page stream
}

func (img *jbig2Image) Subtype() pdf.Name {
	return "Image"
}

func (img *jbig2Image) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	ref := rm.Alloc()
	dict := pdf.Dict{
		"Type":      pdf.Name("XObject"),
		"Subtype":   pdf.Name("Image"),
		"Width":     pdf.Integer(img.width),
		"Height":    pdf.Integer(img.height),
		"Filter":    pdf.Name("JBIG2Decode"),
		"ImageMask": pdf.Boolean(true),
		"Decode":    pdf.Array{pdf.Integer(1), pdf.Integer(0)}, // JBIG2: 1=black
	}

	// embed JBIG2Globals as a separate stream if present
	if img.globals != nil {
		globalsRef := rm.Alloc()
		globalsStream, err := rm.Out().OpenStream(globalsRef, pdf.Dict{})
		if err != nil {
			return nil, err
		}
		if _, err = globalsStream.Write(img.globals); err != nil {
			globalsStream.Close()
			return nil, err
		}
		if err = globalsStream.Close(); err != nil {
			return nil, err
		}

		dict["DecodeParms"] = pdf.Dict{
			"JBIG2Globals": globalsRef,
		}
	}

	// the JBIG2 page data is the stream content
	stream, err := rm.Out().OpenStream(ref, dict)
	if err != nil {
		return nil, err
	}
	if _, err = stream.Write(img.pageData); err != nil {
		stream.Close()
		return nil, err
	}
	return ref, stream.Close()
}

// bitmapToMask converts a bitmap.Bitmap to a standard PDF image mask
// (Flate-compressed for comparison).
func bitmapToMask(bm *bitmap.Bitmap) *pdfimg.Mask {
	return &pdfimg.Mask{
		Width:    bm.Width(),
		Height:   bm.Height(),
		Inverted: true, // 1=opaque (black) in our convention
		WriteData: func(w io.Writer) error {
			stride := (bm.Width() + 7) / 8
			for y := 0; y < bm.Height(); y++ {
				_, err := w.Write(bm.Pix[y*bm.Stride : y*bm.Stride+stride])
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// placement describes a symbol instance for testing.
type placement struct {
	symID int
	x, y  int
}
