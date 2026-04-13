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
	"bytes"
	"fmt"
	goimg "image"
	gocol "image/color"
	"image/jpeg"
	"math"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	pdfimg "seehuhn.de/go/pdf/graphics/image"
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

// stripe colors for the RGB row.
//
// These colors are chosen to be as vivid as possible while ensuring
// that the YCbCr pre-inversion (treating R,G,B as Y,Cb,Cr and applying
// YCbCrToRGB) stays within [0, 255] for all three channels.
var rgbStripes = []gocol.NRGBA{
	{R: 195, G: 75, B: 75, A: 255},  // red
	{R: 200, G: 155, B: 70, A: 255}, // gold
	{R: 80, G: 160, B: 75, A: 255},  // green
	{R: 55, G: 108, B: 208, A: 255}, // blue
}

// stripe colors for the CMYK row (C, M, Y, K).
var cmykStripes = [][4]byte{
	{0, 190, 200, 15}, // red
	{0, 15, 225, 5},   // yellow
	{200, 0, 200, 50}, // green
	{230, 130, 0, 10}, // blue
}

func run(filename string) error {
	const w, h = 64, 64

	rgbImg := createRGBImage(w, h)
	cmykPixels := createCMYKPixels(w, h)

	// pre-inverted RGB JPEG for the ColorTransform=0 case
	preInvRGB := preInvertColors(rgbImg)
	funnyRGBData := encodeJPEG(preInvRGB)

	// 4-component CMYK JPEG (solid-color stripes, no APP14 marker)
	cmykData := encodeCMYKJPEG(w, h, cmykPixels, -1)

	page, err := document.CreateSinglePage(filename, document.A4, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	font := standard.Helvetica.New()
	F := text.F{Font: font, Size: 14}
	Fsmall := text.F{Font: font, Size: 10}

	// title
	text.Show(page.Builder,
		text.M{X: 72, Y: 790},
		F, "DCTDecode ColorTransform Test",
	)

	const (
		imgSize = 170.0 // drawn size of each image
		colL    = 110.0 // left column x
		colR    = 320.0 // right column x
		rowT    = 570.0 // top row image bottom y
		rowB    = 360.0 // bottom row image bottom y
		labelX  = 90.0  // x position of rotated row labels
	)

	// column labels (centered above each image)
	text.Show(page.Builder,
		text.M{X: colL + 45, Y: rowT + imgSize + 15},
		Fsmall, "Normal encoding",
	)
	text.Show(page.Builder,
		text.M{X: colR + 40, Y: rowT + imgSize + 15},
		Fsmall, "Non-default encoding",
	)

	// row labels (rotated 90° CCW, to the left of images)
	drawRotatedLabel(page, Fsmall, "DeviceRGB", labelX, rowT+imgSize/2, 50)
	drawRotatedLabel(page, Fsmall, "DeviceCMYK", labelX, rowB+imgSize/2, 56)

	ct0 := 0

	// top row: DeviceRGB

	// top-left: normal RGB JPEG
	rgbBounds := rgbImg.Bounds()
	normalRGB := &pdfimg.Dict{
		Width:            rgbBounds.Dx(),
		Height:           rgbBounds.Dy(),
		ColorSpace:       color.SpaceDeviceRGB,
		BitsPerComponent: 8,
		Data: &pdfimg.DCTSource{
			Image:   rgbImg,
			Options: &jpeg.Options{Quality: 100},
		},
	}
	drawImage(page, normalRGB, colL, rowT, imgSize)

	// top-right: pre-inverted RGB JPEG with ColorTransform=0
	funnyRGB := &rawJPEGImage{
		data:           funnyRGBData,
		width:          w,
		height:         h,
		colorSpace:     "DeviceRGB",
		colorTransform: &ct0,
	}
	drawImage(page, funnyRGB, colR, rowT, imgSize)

	// bottom row: DeviceCMYK

	// bottom-left: normal CMYK JPEG (raw CMYK, no transform)
	normalCMYK := &rawJPEGImage{
		data:       cmykData,
		width:      w,
		height:     h,
		colorSpace: "DeviceCMYK",
	}
	drawImage(page, normalCMYK, colL, rowB, imgSize)

	// bottom-right: YCCK-encoded CMYK JPEG with APP14 transform=2
	ycckData := encodeCMYKJPEG(w, h, cmykToYCCK(cmykPixels), 2)
	funnyCMYK := &rawJPEGImage{
		data:       ycckData,
		width:      w,
		height:     h,
		colorSpace: "DeviceCMYK",
	}
	drawImage(page, funnyCMYK, colR, rowB, imgSize)

	// footnote
	text.Show(page.Builder,
		text.M{X: 72, Y: rowB - 30},
		Fsmall, "Within each row, both images should look the same.",
	)

	return page.Close()
}

func drawRotatedLabel(page *document.Page, f text.F, label string, x, y, textWidth float64) {
	page.PushGraphicsState()
	page.Transform(matrix.Translate(x, y))
	page.Transform(matrix.Rotate(math.Pi / 2))
	text.Show(page.Builder, text.M{X: -textWidth / 2, Y: 0}, f, label)
	page.PopGraphicsState()
}

func drawImage(page *document.Page, img graphics.XObject, x, y, size float64) {
	page.PushGraphicsState()
	page.Transform(matrix.Translate(x, y))
	page.Transform(matrix.Scale(size, size))
	page.DrawXObject(img)
	page.PopGraphicsState()
}

func createRGBImage(w, h int) *goimg.NRGBA {
	img := goimg.NewNRGBA(goimg.Rect(0, 0, w, h))
	stripeH := h / len(rgbStripes)
	for y := range h {
		idx := min(y/stripeH, len(rgbStripes)-1)
		for x := range w {
			img.SetNRGBA(x, y, rgbStripes[idx])
		}
	}
	return img
}

// cmykToYCCK converts CMYK pixel data to YCCK encoding per Adobe
// Technical Note #5116 section 13.1: apply RGB-to-YCbCr on
// R=(255-C), G=(255-M), B=(255-Y); K is passed through unchanged.
func cmykToYCCK(cmykPixels []byte) []byte {
	ycck := make([]byte, len(cmykPixels))
	for i := 0; i < len(cmykPixels); i += 4 {
		c, m, y, k := cmykPixels[i], cmykPixels[i+1], cmykPixels[i+2], cmykPixels[i+3]
		yy, cb, cr := gocol.RGBToYCbCr(255-c, 255-m, 255-y)
		ycck[i] = yy
		ycck[i+1] = cb
		ycck[i+2] = cr
		ycck[i+3] = k
	}
	return ycck
}

func createCMYKPixels(w, h int) []byte {
	pixels := make([]byte, w*h*4)
	stripeH := h / len(cmykStripes)
	for y := range h {
		idx := min(y/stripeH, len(cmykStripes)-1)
		for x := range w {
			copy(pixels[(y*w+x)*4:], cmykStripes[idx][:])
		}
	}
	return pixels
}

// preInvertColors applies the inverse of the JPEG color transform.
// For each pixel (R, G, B), it computes YCbCrToRGB(R, G, B) where
// R is treated as Y, G as Cb, B as Cr. After JPEG encoding (which
// applies RGB->YCbCr), the stored components will equal (R, G, B).
func preInvertColors(src *goimg.NRGBA) *goimg.NRGBA {
	b := src.Bounds()
	dst := goimg.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := src.NRGBAAt(x, y)
			r, g, b := gocol.YCbCrToRGB(c.R, c.G, c.B)
			dst.SetNRGBA(x, y, gocol.NRGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return dst
}

func encodeJPEG(img goimg.Image) []byte {
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100})
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// rawJPEGImage embeds pre-encoded JPEG bytes as a PDF image XObject.
type rawJPEGImage struct {
	data           []byte
	width, height  int
	colorSpace     pdf.Name
	colorTransform *int
}

func (img *rawJPEGImage) Subtype() pdf.Name {
	return "Image"
}

func (img *rawJPEGImage) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	ref := rm.Alloc()
	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(img.width),
		"Height":           pdf.Integer(img.height),
		"ColorSpace":       img.colorSpace,
		"BitsPerComponent": pdf.Integer(8),
		"Filter":           pdf.Name("DCTDecode"),
	}
	if img.colorTransform != nil {
		dict["DecodeParms"] = pdf.Dict{
			"ColorTransform": pdf.Integer(*img.colorTransform),
		}
	}

	stream, err := rm.Out().OpenStream(ref, dict)
	if err != nil {
		return nil, err
	}
	if _, err = stream.Write(img.data); err != nil {
		stream.Close()
		return nil, err
	}
	return ref, stream.Close()
}

// encodeCMYKJPEG creates a baseline 4-component JPEG from pixel data.
// w and h must be multiples of 8, and each 8x8 block must be solid-colored.
// pixels is w*h*4 bytes of interleaved component data in row-major order.
// If adobeTransform is non-negative, an APP14 marker with that transform
// code is included (0 = raw CMYK, 2 = YCCK).
func encodeCMYKJPEG(w, h int, pixels []byte, adobeTransform int) []byte {
	var buf bytes.Buffer

	buf.Write([]byte{0xFF, 0xD8}) // SOI

	// APP14 (Adobe) marker
	if adobeTransform >= 0 {
		buf.Write([]byte{
			0xFF, 0xEE, // APP14 marker
			0x00, 0x0E, // length = 14
			'A', 'd', 'o', 'b', 'e', // "Adobe"
			0x00, 0x65, // version 101
			0x00, 0x00, // flags0
			0x00, 0x00, // flags1
			byte(adobeTransform), // transform code
		})
	}

	// DQT: quantization table 0, all ones (lossless)
	buf.Write([]byte{0xFF, 0xDB, 0x00, 0x43, 0x00})
	for range 64 {
		buf.WriteByte(1)
	}

	// SOF0: baseline, 8-bit, 4 components, no subsampling
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x14}) // length = 8 + 3*4 = 20
	buf.WriteByte(8)              // precision
	buf.WriteByte(byte(h >> 8))   // height
	buf.WriteByte(byte(h))
	buf.WriteByte(byte(w >> 8)) // width
	buf.WriteByte(byte(w))
	buf.WriteByte(4) // 4 components
	for i := range 4 {
		buf.WriteByte(byte(i + 1)) // component ID
		buf.WriteByte(0x11)        // H=1, V=1
		buf.WriteByte(0x00)        // quant table 0
	}

	// DHT: DC table 0 (standard luminance) + AC table 0 (EOB only)
	dcLi := [16]byte{0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0}
	dcVi := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	acLi := [16]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	acVi := []byte{0x00} // EOB only
	length := 2 + (1 + 16 + len(dcVi)) + (1 + 16 + len(acVi))
	buf.Write([]byte{0xFF, 0xC4, byte(length >> 8), byte(length)})
	buf.WriteByte(0x00) // DC table, id 0
	buf.Write(dcLi[:])
	buf.Write(dcVi)
	buf.WriteByte(0x10) // AC table, id 0
	buf.Write(acLi[:])
	buf.Write(acVi)

	// SOS: 4 components, all using table 0
	buf.Write([]byte{0xFF, 0xDA})
	buf.Write([]byte{0x00, 0x0E}) // length = 6 + 2*4 = 14
	buf.WriteByte(4)
	for i := range 4 {
		buf.WriteByte(byte(i + 1)) // component selector
		buf.WriteByte(0x00)        // DC table 0, AC table 0
	}
	buf.Write([]byte{0x00, 0x3F, 0x00}) // Ss=0, Se=63, Ah=0:Al=0

	// entropy-coded data
	bw := &bitWriter{buf: &buf}
	var prevDC [4]int
	for by := range h / 8 {
		for bx := range w / 8 {
			for comp := range 4 {
				// pixel value from the block's top-left corner
				v := int(pixels[(by*8*w+bx*8)*4+comp])
				dc := 8 * (v - 128)
				diff := dc - prevDC[comp]
				prevDC[comp] = dc

				// encode DC difference
				cat := dcCategory(diff)
				hc := dcCodes[cat]
				bw.writeBits(hc.code, hc.bits)
				if cat > 0 {
					val := diff
					if val < 0 {
						val--
					}
					bw.writeBits(uint32(val)&((1<<cat)-1), cat)
				}

				// encode AC: all zero, just EOB
				bw.writeBits(0, 1)
			}
		}
	}
	bw.flush()

	buf.Write([]byte{0xFF, 0xD9}) // EOI
	return buf.Bytes()
}

func dcCategory(v int) int {
	if v < 0 {
		v = -v
	}
	n := 0
	for v > 0 {
		v >>= 1
		n++
	}
	return n
}

// Huffman codes for the standard DC luminance table (JPEG Table K.3).
var dcCodes = [12]struct {
	code uint32
	bits int
}{
	{0b00, 2},        // category 0
	{0b010, 3},       // category 1
	{0b011, 3},       // category 2
	{0b100, 3},       // category 3
	{0b101, 3},       // category 4
	{0b110, 3},       // category 5
	{0b1110, 4},      // category 6
	{0b11110, 5},     // category 7
	{0b111110, 6},    // category 8
	{0b1111110, 7},   // category 9
	{0b11111110, 8},  // category 10
	{0b111111110, 9}, // category 11
}

// bitWriter writes individual bits with JPEG byte stuffing (FF -> FF 00).
type bitWriter struct {
	buf *bytes.Buffer
	acc uint32
	n   int
}

func (bw *bitWriter) writeBits(val uint32, nbits int) {
	bw.acc = (bw.acc << nbits) | (val & ((1 << nbits) - 1))
	bw.n += nbits
	for bw.n >= 8 {
		bw.n -= 8
		b := byte(bw.acc >> bw.n)
		bw.buf.WriteByte(b)
		if b == 0xFF {
			bw.buf.WriteByte(0x00)
		}
	}
}

func (bw *bitWriter) flush() {
	if bw.n > 0 {
		b := byte((bw.acc << (8 - bw.n)) | ((1 << (8 - bw.n)) - 1))
		bw.buf.WriteByte(b)
		if b == 0xFF {
			bw.buf.WriteByte(0x00)
		}
		bw.n = 0
		bw.acc = 0
	}
}
