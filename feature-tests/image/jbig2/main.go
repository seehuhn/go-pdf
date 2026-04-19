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

	// page 2: generic region (arithmetic)
	if err := pageGenericDefault(doc, font); err != nil {
		return err
	}

	// page 3: generic region (MMR)
	if err := pageGenericMMR(doc, font); err != nil {
		return err
	}

	// page 4: generic region (typical prediction)
	if err := pageGenericTPGD(doc, font); err != nil {
		return err
	}

	// page 5: text region (arithmetic)
	if err := pageTextArithmetic(doc, font); err != nil {
		return err
	}

	// page 6: text region (Huffman)
	if err := pageTextHuffman(doc, font); err != nil {
		return err
	}

	// page 7: text region with page-local symbols
	if err := pageTextLocal(doc, font); err != nil {
		return err
	}

	// page 8: refinement region
	if err := pageRefinement(doc, font); err != nil {
		return err
	}

	// page 9: halftone region (arithmetic)
	if err := pageHalftone(doc, font); err != nil {
		return err
	}

	// page 10: halftone region (MMR region, arithmetic pattern dict)
	if err := pageHalftoneMMR(doc, font); err != nil {
		return err
	}

	// page 11: halftone region (MMR pattern dict, arithmetic region)
	if err := pageHalftonePatternMMR(doc, font); err != nil {
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
	if err := im.AddGenericRegion(bm, 0, 0, nil); err != nil {
		return err
	}
	jbig2Img := newJBIG2Image(im, 32, 32)
	refImg := bitmapToGrayImage(bm)

	return drawPage(doc, font,
		"Colour handling sanity check",
		"A fully-black 1-bit bitmap is embedded as a JBIG2 stream with ColorSpace DeviceGray and default Decode.  The left square must render as solid black: if the JBIG2Decode output is accidentally inverted, or the Decode array is wrong, this is where it shows up first.",
		jbig2Img, refImg, 32, 32)
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
	if err := im.AddGenericRegion(bm, 0, 0, nil); err != nil {
		return err
	}

	jbig2Mask := newJBIG2Image(im, bm.Width(), bm.Height())
	refMask := bitmapToGrayImage(bm)
	return drawPage(doc, font,
		"Generic region — arithmetic coder",
		"A single generic region segment carries the entire bitmap, coded with the default arithmetic coder (template 0, no typical prediction).  This is the baseline JBIG2 coding path and exercises segment framing, page-information handling, and the arithmetic decoder.",
		jbig2Mask, refMask,
		float64(bm.Width()), float64(bm.Height()))
}

// pageGenericMMR: generic region with MMR coding.
func pageGenericMMR(doc *document.MultiPage, font *type1.Instance) error {
	bm := concentricRects()

	im := jbig2.NewImage(bm.Width(), bm.Height(), nil)
	if err := im.AddGenericRegion(bm, 0, 0, &jbig2.GenericOptions{UseMMR: true}); err != nil {
		return err
	}

	jbig2Mask := newJBIG2Image(im, bm.Width(), bm.Height())
	refMask := bitmapToGrayImage(bm)
	return drawPage(doc, font,
		"Generic region — MMR coder",
		"The same bitmap coded with the MMR (modified-modified-READ, Group-4 facsimile) coder, selected via GenericOptions.UseMMR.  MMR is stateless between segments and well suited to simple two-tone images; the arithmetic coder's adaptive context machinery is bypassed entirely.",
		jbig2Mask, refMask,
		float64(bm.Width()), float64(bm.Height()))
}

// pageGenericTPGD: generic region with typical prediction.
func pageGenericTPGD(doc *document.MultiPage, font *type1.Instance) error {
	bm := checkerboard()

	im := jbig2.NewImage(bm.Width(), bm.Height(), nil)
	if err := im.AddGenericRegion(bm, 0, 0, &jbig2.GenericOptions{TPGDOn: true}); err != nil {
		return err
	}

	jbig2Mask := newJBIG2Image(im, bm.Width(), bm.Height())
	refMask := bitmapToGrayImage(bm)
	return drawPage(doc, font,
		"Generic region — typical prediction",
		"GenericOptions.TPGDOn enables the typical-prediction optimisation: rows identical to the row above are flagged rather than re-coded.  The decoder must honour the TP flag on a per-row basis to reconstruct the original image.  A checkerboard exercises both predicted and non-predicted rows.",
		jbig2Mask, refMask,
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
	if err := im.AddTextRegion(&jbig2.TextRegion{
		Width:     regionW,
		Height:    regionH,
		Instances: instances,
	}); err != nil {
		return err
	}

	jbig2Mask := newJBIG2Image(im, regionW, regionH)
	refBm := renderSymbolBitmap(symbols, pp, regionW, regionH)
	refMask := bitmapToGrayImage(refBm)
	return drawPage(doc, font,
		"Text region — shared symbols, arithmetic coder",
		"A shared Globals stream holds the symbol dictionary; the page references it through a text region.  Each instance places a symbol at a reference point.  Symbol IDs, instance positions, and the strip structure are all encoded with the arithmetic coder.",
		jbig2Mask, refMask,
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
	if err := im.AddTextRegion(&jbig2.TextRegion{
		Width:      regionW,
		Height:     regionH,
		Instances:  instances,
		UseHuffman: true,
	}); err != nil {
		return err
	}

	jbig2Mask := newJBIG2Image(im, regionW, regionH)
	refBm := renderSymbolBitmap(symbols, pp, regionW, regionH)
	refMask := bitmapToGrayImage(refBm)
	return drawPage(doc, font,
		"Text region — shared symbols, Huffman coder",
		"The same symbol placements coded with TextRegion.UseHuffman=true.  Instance positions and symbol IDs are coded against the fixed JBIG2 Huffman tables (Annex B).  This typically produces larger output than the arithmetic coder, but the decoder is simpler because no adaptive contexts or probability estimation are involved.",
		jbig2Mask, refMask,
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
	if err := im.AddRefinementRegion(target, ref, 0, 0, nil); err != nil {
		return err
	}

	jbig2Img := newJBIG2Image(im, target.Width(), target.Height())
	refImg := bitmapToGrayImage(target)
	return drawPage(doc, font,
		"Generic refinement region",
		"The target bitmap is coded as a refinement of a slightly different reference.  The reference is first written as a generic region so the decoder places it into the page buffer; the refinement region then overrides pixels using adaptive predictions against the reference.",
		jbig2Img, refImg,
		float64(target.Width()), float64(target.Height()))
}

// halftoneFixture holds a reusable 8×6 Bayer halftone fixture shared
// by the three halftone pages: an eight-entry pattern dictionary, a
// gray-value grid covering every pattern index, and the bitmap that
// the JBIG2 output must match.
type halftoneFixture struct {
	patW, patH       int
	gridW, gridH     int
	regionW, regionH int
	patterns         []*bitmap.Bitmap
	grayValues       []int
	refBm            *bitmap.Bitmap
}

// makeHalftoneFixture builds the shared halftone fixture: eight 4×4
// Bayer patterns at gray levels 0..7, tiled into an 8×6 grid.  The
// power-of-two pattern count avoids out-of-range gray indices in the
// bitplane encoding.
func makeHalftoneFixture() *halftoneFixture {
	const patW, patH = 4, 4
	const numPatterns = 8
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

	const gridW, gridH = 8, 6
	grayValues := make([]int, gridW*gridH)
	for gy := range gridH {
		for gx := range gridW {
			grayValues[gy*gridW+gx] = (gx + gy*gridW) % numPatterns
		}
	}

	const regionW = gridW * patW
	const regionH = gridH * patH

	// reference bitmap: grid vector (patW, 0), row vector (0, patH)
	refBm := bitmap.New(regionW, regionH)
	for gy := range gridH {
		for gx := range gridW {
			pat := patterns[grayValues[gy*gridW+gx]]
			refBm.Combine(pat, gx*patW, gy*patH, bitmap.CombOpOR)
		}
	}

	return &halftoneFixture{
		patW: patW, patH: patH,
		gridW: gridW, gridH: gridH,
		regionW: regionW, regionH: regionH,
		patterns:   patterns,
		grayValues: grayValues,
		refBm:      refBm,
	}
}

// pageHalftone: halftone region with a pattern dictionary.
func pageHalftone(doc *document.MultiPage, font *type1.Instance) error {
	f := makeHalftoneFixture()

	g := jbig2.NewGlobals()
	patID, err := g.AddPatternDict(f.patterns, nil)
	if err != nil {
		return err
	}

	im := jbig2.NewImage(f.regionW, f.regionH, g)
	if err := im.AddHalftoneRegion(&jbig2.HalftoneRegion{
		Width:         f.regionW,
		Height:        f.regionH,
		PatternDictID: patID,
		GrayValues:    f.grayValues,
		GridWidth:     f.gridW,
		GridHeight:    f.gridH,
		GridVX:        f.patW,
	}); err != nil {
		return err
	}

	jbig2Mask := newJBIG2Image(im, f.regionW, f.regionH)
	refMask := bitmapToGrayImage(f.refBm)

	return drawPage(doc, font,
		"Halftone region — arithmetic coder",
		"A pattern dictionary holds eight same-size dither patterns (Bayer-style).  The halftone region tiles the page with patterns chosen from an 8×6 gray-value grid; the bitplanes of the gray indices are coded with the arithmetic coder against a template.",
		jbig2Mask, refMask,
		float64(f.regionW), float64(f.regionH))
}

// pageHalftoneMMR: halftone region with MMR coding of the gray-scale
// bitplanes.
func pageHalftoneMMR(doc *document.MultiPage, font *type1.Instance) error {
	f := makeHalftoneFixture()

	g := jbig2.NewGlobals()
	patID, err := g.AddPatternDict(f.patterns, nil)
	if err != nil {
		return err
	}

	im := jbig2.NewImage(f.regionW, f.regionH, g)
	if err := im.AddHalftoneRegion(&jbig2.HalftoneRegion{
		Width:         f.regionW,
		Height:        f.regionH,
		PatternDictID: patID,
		GrayValues:    f.grayValues,
		GridWidth:     f.gridW,
		GridHeight:    f.gridH,
		GridVX:        f.patW,
		UseMMR:        true,
	}); err != nil {
		return err
	}

	jbig2Mask := newJBIG2Image(im, f.regionW, f.regionH)
	refMask := bitmapToGrayImage(f.refBm)

	return drawPage(doc, font,
		"Halftone region — MMR coder",
		"The same halftone with HalftoneRegion.UseMMR=true.  The gray-scale bitplanes of the pattern-index grid are packed into an auxiliary bitmap and compressed with MMR instead of the arithmetic coder.  The pattern dictionary itself still uses the arithmetic coder.",
		jbig2Mask, refMask,
		float64(f.regionW), float64(f.regionH))
}

// pageHalftonePatternMMR: halftone region with an MMR-coded pattern
// dictionary.  The halftone region itself uses the arithmetic coder,
// so this page isolates the PatternDictOptions.UseMMR path.
func pageHalftonePatternMMR(doc *document.MultiPage, font *type1.Instance) error {
	f := makeHalftoneFixture()

	g := jbig2.NewGlobals()
	patID, err := g.AddPatternDict(f.patterns, &jbig2.PatternDictOptions{UseMMR: true})
	if err != nil {
		return err
	}

	im := jbig2.NewImage(f.regionW, f.regionH, g)
	if err := im.AddHalftoneRegion(&jbig2.HalftoneRegion{
		Width:         f.regionW,
		Height:        f.regionH,
		PatternDictID: patID,
		GrayValues:    f.grayValues,
		GridWidth:     f.gridW,
		GridHeight:    f.gridH,
		GridVX:        f.patW,
	}); err != nil {
		return err
	}

	jbig2Mask := newJBIG2Image(im, f.regionW, f.regionH)
	refMask := bitmapToGrayImage(f.refBm)

	return drawPage(doc, font,
		"Halftone region — MMR pattern dictionary",
		"The halftone region uses the arithmetic coder, but its pattern dictionary is stored with PatternDictOptions.UseMMR=true.  The dictionary's patterns are tiled into a single wide collective bitmap and compressed with MMR; the halftone's own bitplane encoding is unchanged.",
		jbig2Mask, refMask,
		float64(f.regionW), float64(f.regionH))
}

// pageTextLocal: text region that references a page-local symbol
// dictionary (Image.AddSymbol) instead of a shared globals dictionary.
func pageTextLocal(doc *document.MultiPage, font *type1.Instance) error {
	symbols := makeSymbols()
	pp := symbolPlacements()
	const regionW, regionH = 80, 40

	im := jbig2.NewImage(regionW, regionH, nil)
	for _, sym := range symbols {
		if _, err := im.AddSymbol(sym); err != nil {
			return err
		}
	}

	instances := placementsToInstances(pp)
	for i := range instances {
		instances[i].Local = true
	}
	if err := im.AddTextRegion(&jbig2.TextRegion{
		Width:     regionW,
		Height:    regionH,
		Instances: instances,
	}); err != nil {
		return err
	}

	jbig2Mask := newJBIG2Image(im, regionW, regionH)
	refBm := renderSymbolBitmap(symbols, pp, regionW, regionH)
	refMask := bitmapToGrayImage(refBm)
	return drawPage(doc, font,
		"Text region — page-local symbols",
		"Symbols added via Image.AddSymbol form a symbol dictionary written inside the image stream itself; instances reference them with TextRegionInstance.Local=true.  No globals stream is embedded, so the PDF DecodeParms entry is omitted.",
		jbig2Mask, refMask,
		float64(regionW), float64(regionH))
}

// drawPage draws a single test page with a JBIG2 image on the left
// and a reference image on the right.  title names the feature under
// test and description explains it in prose below the images.
func drawPage(
	doc *document.MultiPage,
	font *type1.Instance,
	title, description string,
	jbig2Img, refImg graphics.XObject,
	imgPixW, imgPixH float64,
) error {
	page := doc.AddPage()

	fTitle := text.F{Font: font, Size: 14}
	fLabel := text.F{Font: font, Size: 9}
	fCaption := text.F{Font: font, Size: 10}
	fBody := text.F{Font: font, Size: 9}

	// A5r dimensions: 595.276 x 420.945
	const (
		leftX  = 40.0
		rightX = 320.0
		titleY = 395.0
		labelY = 375.0
		bodyW  = 515.0
	)

	// scale images to 180pt wide, maintaining aspect ratio
	const dispW = 180.0
	dispH := dispW * imgPixH / imgPixW
	imgY := labelY - dispH - 10

	// title
	text.Show(page.Builder, text.M{X: leftX, Y: titleY}, fTitle, title)

	// left: JBIG2
	text.Show(page.Builder, text.M{X: leftX, Y: labelY}, fLabel, "JBIG2 encoded")
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

	// caption
	captionY := imgY - 18
	text.Show(page.Builder, text.M{X: leftX, Y: captionY}, fCaption,
		"Both images should look identical.")

	// description paragraph, wrapped
	descY := captionY - 18
	text.Show(page.Builder, text.M{X: leftX, Y: descY}, fBody,
		text.Wrap(bodyW, description))

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
