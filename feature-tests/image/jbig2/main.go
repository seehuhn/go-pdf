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
	"seehuhn.de/go/pdf/graphics/color"
	pdfimg "seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/graphics/image/jbig2"
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
	doc, err := document.CreateMultiPage(filename, document.A5r, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	font := standard.Helvetica.New()

	// page 1: colour sanity check
	if err := pageColourCheck(doc, font); err != nil {
		return err
	}

	// page 2: generic region (default)
	if err := pageGenericDefault(doc, font); err != nil {
		return err
	}

	// page 2: generic region (MMR)
	if err := pageGenericMMR(doc, font); err != nil {
		return err
	}

	// page 3: generic region (typical prediction)
	if err := pageGenericTPGD(doc, font); err != nil {
		return err
	}

	// page 4: text region (arithmetic)
	if err := pageTextArithmetic(doc, font); err != nil {
		return err
	}

	// page 5: text region (Huffman)
	if err := pageTextHuffman(doc, font); err != nil {
		return err
	}

	// page 6: refinement region
	if err := pageRefinement(doc, font); err != nil {
		return err
	}

	// page 7: halftone region
	if err := pageHalftone(doc, font); err != nil {
		return err
	}

	return doc.Close()
}

// pageColourCheck draws an all-black square as a sanity check for
// correct colour handling.
func pageColourCheck(doc *document.MultiPage, font *type1.Instance) error {
	bm := bitmap.New(32, 32)
	for y := range 32 {
		for x := range 32 {
			bm.SetPixel(x, y, true)
		}
	}

	im := jbig2.NewImage(32, 32, nil)
	im.AddGenericRegion(bm, 0, 0, nil)
	jbig2Img := newJBIG2Image(im, 32, 32)
	refImg := bitmapToGrayImage(bm)

	return drawPage(doc, font, "solid black square", jbig2Img, refImg, 32, 32)
}

// concentricRects creates a 64x64 bitmap with concentric rectangles.
func concentricRects() *bitmap.Bitmap {
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
	return bm
}

// checkerboard creates a 64x64 checkerboard bitmap.
func checkerboard() *bitmap.Bitmap {
	bm := bitmap.New(64, 64)
	for y := range 64 {
		for x := range 64 {
			if (x/4+y/4)%2 == 0 {
				bm.SetPixel(x, y, true)
			}
		}
	}
	return bm
}

// makeSymbols creates three 8x8 symbol bitmaps: square, cross, diamond.
func makeSymbols() []*bitmap.Bitmap {
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

	return []*bitmap.Bitmap{symSquare, symCross, symDiamond}
}

// symbolPlacements returns the placements for a grid of symbols.
func symbolPlacements() []placement {
	var pp []placement
	idx := 0
	for row := range 4 {
		for col := range 8 {
			pp = append(pp, placement{
				symID: idx % 3,
				x:     col * 10,
				y:     (row+1)*8 - 1,
			})
			idx++
		}
	}
	return pp
}

// renderSymbolBitmap renders the symbol placements onto a fresh bitmap.
func renderSymbolBitmap(symbols []*bitmap.Bitmap, pp []placement, w, h int) *bitmap.Bitmap {
	bm := bitmap.New(w, h)
	for _, p := range pp {
		sym := symbols[p.symID]
		topY := p.y - sym.Height() + 1
		bm.Combine(sym, p.x, topY, bitmap.CombOpOR)
	}
	return bm
}

// pageGenericDefault: generic region with default options.
func pageGenericDefault(doc *document.MultiPage, font *type1.Instance) error {
	bm := concentricRects()

	im := jbig2.NewImage(bm.Width(), bm.Height(), nil)
	im.AddGenericRegion(bm, 0, 0, nil)

	jbig2Mask := newJBIG2Image(im, bm.Width(), bm.Height())
	refMask := bitmapToGrayImage(bm)
	return drawPage(doc, font, "generic region", jbig2Mask, refMask,
		float64(bm.Width()), float64(bm.Height()))
}

// pageGenericMMR: generic region with MMR coding.
func pageGenericMMR(doc *document.MultiPage, font *type1.Instance) error {
	bm := concentricRects()

	im := jbig2.NewImage(bm.Width(), bm.Height(), nil)
	im.AddGenericRegion(bm, 0, 0, &jbig2.GenericOptions{UseMMR: true})

	jbig2Mask := newJBIG2Image(im, bm.Width(), bm.Height())
	refMask := bitmapToGrayImage(bm)
	return drawPage(doc, font, "generic region (MMR)", jbig2Mask, refMask,
		float64(bm.Width()), float64(bm.Height()))
}

// pageGenericTPGD: generic region with typical prediction.
func pageGenericTPGD(doc *document.MultiPage, font *type1.Instance) error {
	bm := checkerboard()

	im := jbig2.NewImage(bm.Width(), bm.Height(), nil)
	im.AddGenericRegion(bm, 0, 0, &jbig2.GenericOptions{TPGDOn: true})

	jbig2Mask := newJBIG2Image(im, bm.Width(), bm.Height())
	refMask := bitmapToGrayImage(bm)
	return drawPage(doc, font, "generic region (typical prediction)", jbig2Mask, refMask,
		float64(bm.Width()), float64(bm.Height()))
}

// pageTextArithmetic: text region with arithmetic coding.
func pageTextArithmetic(doc *document.MultiPage, font *type1.Instance) error {
	symbols := makeSymbols()
	pp := symbolPlacements()
	const regionW, regionH = 80, 40

	g := jbig2.NewGlobals()
	for _, sym := range symbols {
		if _, err := g.AddSymbol(sym); err != nil {
			return err
		}
	}

	instances := placementsToInstances(pp)
	im := jbig2.NewImage(regionW, regionH, g)
	im.AddTextRegion(&jbig2.TextRegion{
		Width:     regionW,
		Height:    regionH,
		Instances: instances,
	})

	jbig2Mask := newJBIG2Image(im, regionW, regionH)
	refBm := renderSymbolBitmap(symbols, pp, regionW, regionH)
	refMask := bitmapToGrayImage(refBm)
	return drawPage(doc, font, "text region (arithmetic)", jbig2Mask, refMask,
		float64(regionW), float64(regionH))
}

// pageTextHuffman: text region with Huffman coding.
func pageTextHuffman(doc *document.MultiPage, font *type1.Instance) error {
	symbols := makeSymbols()
	pp := symbolPlacements()
	const regionW, regionH = 80, 40

	g := jbig2.NewGlobals()
	for _, sym := range symbols {
		if _, err := g.AddSymbol(sym); err != nil {
			return err
		}
	}

	instances := placementsToInstances(pp)
	im := jbig2.NewImage(regionW, regionH, g)
	im.AddTextRegion(&jbig2.TextRegion{
		Width:      regionW,
		Height:     regionH,
		Instances:  instances,
		UseHuffman: true,
	})

	jbig2Mask := newJBIG2Image(im, regionW, regionH)
	refBm := renderSymbolBitmap(symbols, pp, regionW, regionH)
	refMask := bitmapToGrayImage(refBm)
	return drawPage(doc, font, "text region (Huffman)", jbig2Mask, refMask,
		float64(regionW), float64(regionH))
}

// pageRefinement: refinement region encoding.
// The target bitmap is encoded as a refinement of a similar but
// slightly different reference (shifted by 1 pixel).
func pageRefinement(doc *document.MultiPage, font *type1.Instance) error {
	target := concentricRects()

	// reference: same pattern shifted by 1 pixel
	ref := bitmap.New(64, 64)
	for ring := 0; ring < 32; ring += 4 {
		for x := ring; x < 64-ring; x++ {
			ref.SetPixel(x, ring+1, true)
			ref.SetPixel(x, 63-ring, true)
		}
		for y := ring; y < 64-ring; y++ {
			ref.SetPixel(ring+1, y, true)
			ref.SetPixel(63-ring, y, true)
		}
	}

	im := jbig2.NewImage(target.Width(), target.Height(), nil)
	im.AddRefinementRegion(target, ref, 0, 0, nil)

	jbig2Img := newJBIG2Image(im, target.Width(), target.Height())
	refImg := bitmapToGrayImage(target)
	return drawPage(doc, font, "refinement region", jbig2Img, refImg,
		float64(target.Width()), float64(target.Height()))
}

// pageHalftone: halftone region with a pattern dictionary.
func pageHalftone(doc *document.MultiPage, font *type1.Instance) error {
	// pattern dictionary: 8 patterns of 4x4 pixels (gray levels 0..7).
	// Using a power-of-two count avoids out-of-range gray indices in
	// the bitplane encoding.
	const patW, patH = 4, 4
	numPatterns := 8
	// Bayer dither threshold matrix for 4x4 patterns.
	threshold := [16]int{
		0, 8, 2, 10,
		12, 4, 14, 6,
		3, 11, 1, 9,
		15, 7, 13, 5,
	}
	patterns := make([]*bitmap.Bitmap, numPatterns)
	for i := range numPatterns {
		p := bitmap.New(patW, patH)
		count := i * 2 // 0, 2, 4, 6, 8, 10, 12, 14 black pixels out of 16
		for y := range patH {
			for x := range patW {
				if threshold[y*patW+x] < count {
					p.SetPixel(x, y, true)
				}
			}
		}
		patterns[i] = p
	}

	g := jbig2.NewGlobals()
	patID, err := g.AddPatternDict(patterns)
	if err != nil {
		return err
	}

	// 8x6 gray-scale grid
	const gridW, gridH = 8, 6
	grayValues := make([]int, gridW*gridH)
	for gy := range gridH {
		for gx := range gridW {
			grayValues[gy*gridW+gx] = (gx + gy*gridW) % numPatterns
		}
	}

	const regionW = gridW * patW
	const regionH = gridH * patH

	im := jbig2.NewImage(regionW, regionH, g)
	im.AddHalftoneRegion(&jbig2.HalftoneRegion{
		Width:         regionW,
		Height:        regionH,
		PatternDictID: patID,
		GrayValues:    grayValues,
		GridWidth:     gridW,
		GridHeight:    gridH,
		GridX:         0,
		GridY:         0,
		GridVX:        patW,
		GridVY:        0,
	})

	jbig2Mask := newJBIG2Image(im, regionW, regionH)

	// manually render the halftone as the reference
	refBm := bitmap.New(regionW, regionH)
	for gy := range gridH {
		for gx := range gridW {
			gv := grayValues[gy*gridW+gx]
			pat := patterns[gv]
			// grid vector: column=(patW,0), row=(0,patH)
			px := gx * patW
			py := gy * patH
			refBm.Combine(pat, px, py, bitmap.CombOpOR)
		}
	}
	refMask := bitmapToGrayImage(refBm)

	return drawPage(doc, font, "halftone region", jbig2Mask, refMask,
		float64(regionW), float64(regionH))
}

// drawPage draws a single test page with a JBIG2 image on the left
// and a reference image on the right.
func drawPage(
	doc *document.MultiPage,
	font *type1.Instance,
	jbig2Label string,
	jbig2Img, refImg graphics.XObject,
	imgPixW, imgPixH float64,
) error {
	page := doc.AddPage()

	fTitle := text.F{Font: font, Size: 14}
	fLabel := text.F{Font: font, Size: 9}

	// A5r dimensions: 595.276 x 420.945
	const (
		leftX  = 40.0
		rightX = 320.0
		titleY = 390.0
		labelY = 370.0
	)

	// scale images to ~180pt wide, maintaining aspect ratio
	dispW := 180.0
	dispH := dispW * imgPixH / imgPixW
	if dispH > 280 {
		dispH = 280
		dispW = dispH * imgPixW / imgPixH
	}
	imgY := labelY - dispH - 10

	// title
	text.Show(page.Builder, text.M{X: leftX, Y: titleY}, fTitle,
		"Both images should look identical.")

	// left: JBIG2
	text.Show(page.Builder, text.M{X: leftX, Y: labelY}, fLabel, jbig2Label)
	page.PushGraphicsState()
	page.Transform(matrix.Translate(leftX, imgY))
	page.Transform(matrix.Scale(dispW, dispH))
	page.DrawXObject(jbig2Img)
	page.PopGraphicsState()

	// right: reference
	text.Show(page.Builder, text.M{X: rightX, Y: labelY}, fLabel, "reference")
	page.PushGraphicsState()
	page.Transform(matrix.Translate(rightX, imgY))
	page.Transform(matrix.Scale(dispW, dispH))
	page.DrawXObject(refImg)
	page.PopGraphicsState()

	return page.Close()
}

// newJBIG2Image wraps a jbig2.Image as a 1-bit DeviceGray image.
// The JBIG2Decode filter produces 0=black (matching the normal PDF
// convention), so default Decode [0 1] works correctly.
func newJBIG2Image(im *jbig2.Image, w, h int) *pdfimg.Dict {
	return &pdfimg.Dict{
		Width:            w,
		Height:           h,
		ColorSpace:       color.SpaceDeviceGray,
		BitsPerComponent: 1,
		Data:             im,
	}
}

// bitmapToGrayImage converts a bitmap to a 1-bit DeviceGray image.
// This produces an opaque black-and-white image without image mask
// semantics, providing a clean reference independent of mask conventions.
func bitmapToGrayImage(bm *bitmap.Bitmap) *pdfimg.Dict {
	return &pdfimg.Dict{
		Width:            bm.Width(),
		Height:           bm.Height(),
		ColorSpace:       color.SpaceDeviceGray,
		BitsPerComponent: 1,
		Decode:           []float64{1, 0}, // bitmap 1=black → gray 0.0=black
		Data: &pdfimg.FlateSource{
			Width:            bm.Width(),
			Colors:           1,
			BitsPerComponent: 1,
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
		},
	}
}

// placementsToInstances converts placement data to JBIG2 text region instances.
func placementsToInstances(pp []placement) []jbig2.TextRegionInstance {
	instances := make([]jbig2.TextRegionInstance, len(pp))
	for i, p := range pp {
		instances[i] = jbig2.TextRegionInstance{
			SymbolID: p.symID,
			X:        p.x,
			Y:        p.y,
		}
	}
	return instances
}

// placement describes a symbol instance position.
type placement struct {
	symID int
	x, y  int
}
