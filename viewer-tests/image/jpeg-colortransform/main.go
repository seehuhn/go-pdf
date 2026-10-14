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

// Command jpeg-colortransform generates a one-page PDF to probe how PDF
// viewers pick the colour transform of DCTDecode images.
//
// Each row shows a JPEG image as decoded by the viewer (left) and a Flate
// image holding Quire's decode of the same data (right).  The JPEGs differ
// in the Adobe APP14 marker, the JFIF marker, the JPEG component IDs and the
// /ColorTransform entry.  Where left and right differ, the viewer picks a
// different colour transform from Quire.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	gocol "image/color"
	"io"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	pdfimg "seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/graphics/text"
	"seehuhn.de/go/pdf/internal/filter/dct"
)

const (
	w, h = 48, 32 // image size in samples

	imgW, imgH = 72.0, 48.0 // drawn size in points
	rowPitch   = 58.0
	firstRowY  = 660.0 // bottom of the first image
	labelX     = 30.0
	dctX       = 360.0
	refX       = 460.0
)

// stripe colors for the RGB rows
var rgbStripes = []gocol.NRGBA{
	{R: 195, G: 75, B: 75, A: 255},  // red
	{R: 200, G: 155, B: 70, A: 255}, // gold
	{R: 80, G: 160, B: 75, A: 255},  // green
	{R: 55, G: 108, B: 208, A: 255}, // blue
}

// stripe colors for the CMYK rows (C, M, Y, K)
var cmykStripes = [][4]byte{
	{0, 190, 200, 15}, // red
	{0, 15, 225, 5},   // yellow
	{200, 0, 200, 50}, // green
	{230, 130, 0, 10}, // blue
}

// marks an absent /ColorTransform entry
const absent = -1

// A row describes one test image.
type row struct {
	comps     int      // 3 (RGB) or 4 (CMYK)
	transform bool     // store the data as YCbCr/YCbCrK
	adobe     bool     // keep the Adobe APP14 marker
	jfif      bool     // add a JFIF marker
	ids       []byte   // component IDs, or nil for 1..n
	entry     int      // /ColorTransform entry, or absent
	label     []string // text lines
}

var rows = []row{
	{3, true, false, false, nil, absent, []string{
		"1. RGB, stored as YCbCr; no markers, IDs 1,2,3; no entry",
		"Quire applies the default transform: original colours."}},
	{3, false, false, false, []byte("RGB"), absent, []string{
		"2. RGB, stored raw; no markers, IDs R,G,B; no entry",
		"Quire ignores the component IDs and applies the default transform,",
		"as the spec requires, so its colours look WRONG.  libjpeg- and",
		"pdf.js-based viewers follow the IDs and show the original colours."}},
	{3, false, false, true, nil, 0, []string{
		"3. RGB, stored raw; JFIF marker; entry 0",
		"Quire: the entry beats JFIF, giving the original colours."}},
	{3, false, true, false, nil, 1, []string{
		"4. RGB, stored raw; Adobe APP14 = 0; entry 1",
		"Quire: the marker beats the entry, giving the original colours."}},
	{3, true, true, false, nil, 0, []string{
		"5. RGB, stored as YCbCr; Adobe APP14 = 1; entry 0",
		"Quire: the marker beats the entry, giving the original colours."}},
	{3, false, true, true, nil, absent, []string{
		"6. RGB, stored raw; JFIF + Adobe APP14 = 0; no entry",
		"Quire: the marker beats JFIF, giving the original colours."}},
	{3, false, false, false, nil, 0, []string{
		"7. RGB, stored raw; no markers; entry 0",
		"Quire applies the entry: original colours."}},
	{4, false, false, false, nil, absent, []string{
		"8. CMYK, stored raw; no markers; no entry",
		"Quire applies the default, no transform: original colours."}},
	{4, true, false, false, nil, 1, []string{
		"9. CMYK, stored as YCbCrK; no markers; entry 1",
		"Quire applies the entry: original colours."}},
	{4, true, true, false, nil, 0, []string{
		"10. CMYK, stored as YCbCrK; Adobe APP14 = 2; entry 0",
		"Quire: the marker beats the entry, giving the original colours."}},
	{4, false, true, false, nil, 1, []string{
		"11. CMYK, stored raw; Adobe APP14 = 0; entry 1",
		"Quire: the marker beats the entry, giving the original colours."}},
}

func main() {
	err := run("test.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("wrote test.pdf")
}

func run(filename string) error {
	page, err := document.CreateSinglePage(filename, document.A4, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	helv := font.Must(standard.Helvetica.New())
	helvBold := font.Must(standard.HelveticaBold.New())
	title := text.F{Font: helvBold, Size: 13}
	body := text.F{Font: helv, Size: 9}
	head := text.F{Font: helvBold, Size: 10}
	rowHead := text.F{Font: helvBold, Size: 8}
	small := text.F{Font: helv, Size: 8}

	showLines(page, title, labelX, 805, 14, "JPEG images: which colour transform applies?")
	showLines(page, body, labelX, 785, 11,
		"Left: a JPEG image (DCTDecode), decoded by your viewer.",
		"Right: the same image as decoded by Quire, stored losslessly (FlateDecode).",
		"Quire follows the PDF spec: an Adobe APP14 marker decides, otherwise the /ColorTransform entry,",
		"otherwise the default (transform for three components, none for four).  The JFIF marker and the",
		"JPEG component IDs play no part.  Where the two images differ, your viewer decides differently.",
		"The images are meant to show four stripes: red, gold/yellow, green, blue.")
	showLines(page, head, dctX, firstRowY+imgH+6, 10, "your viewer")
	showLines(page, head, refX, firstRowY+imgH+6, 10, "Quire")

	for i, r := range rows {
		y := firstRowY - float64(i)*rowPitch
		showLines(page, rowHead, labelX, y+imgH-8, 10, r.label[0])
		showLines(page, small, labelX, y+imgH-18, 10, r.label[1:]...)

		jpg, ref, err := r.build()
		if err != nil {
			return err
		}
		cs := r.space()
		dctImg := &rawJPEGImage{data: jpg, width: w, height: h, colorSpace: r.spaceName(), entry: r.entry}
		drawImage(page, dctImg, dctX, y)

		src := pdfimg.NewFlateSource(w, cs, 8, func(wr io.Writer) error {
			_, err := wr.Write(ref)
			return err
		})
		drawImage(page, &pdfimg.Dict{
			Width:            w,
			Height:           h,
			ColorSpace:       cs,
			BitsPerComponent: 8,
			Data:             src,
		}, refX, y)
	}

	return page.Close()
}

// showLines writes lines of text, the first baseline at (x, y).
func showLines(page *document.Page, f text.F, x, y, leading float64, lines ...string) {
	for i, l := range lines {
		text.Show(page.Builder, text.M{X: x, Y: y - float64(i)*leading}, f, l)
	}
}

func drawImage(page *document.Page, img graphics.XObject, x, y float64) {
	page.PushGraphicsState()
	page.Transform(matrix.Matrix{imgW, 0, 0, imgH, x, y})
	page.DrawXObject(img)
	page.PopGraphicsState()
}

func (r row) spaceName() pdf.Name {
	if r.comps == 4 {
		return "DeviceCMYK"
	}
	return "DeviceRGB"
}

func (r row) space() color.Space {
	if r.comps == 4 {
		return color.SpaceDeviceCMYK
	}
	return color.SpaceDeviceRGB
}

// build returns the JPEG bytes for the row, and the samples obtained by
// decoding them with go-pdf.
func (r row) build() (jpg, ref []byte, err error) {
	samples := r.samples()
	var buf bytes.Buffer
	err = dct.Encode(&buf, bytes.NewReader(samples), dct.EncodeParams{
		Width: w, Height: h, Components: r.comps,
		Transform: r.transform, Quality: 100,
	})
	if err != nil {
		return nil, nil, err
	}
	jpg = buf.Bytes()
	if !r.adobe {
		jpg = stripAPP14(jpg)
	}
	if r.jfif {
		jpg = insertJFIF(jpg)
	}
	if r.ids != nil {
		jpg = setComponentIDs(jpg, r.ids)
	}

	ct := pdf.DCTColorTransformAuto
	switch r.entry {
	case 0:
		ct = pdf.DCTColorTransformNone
	case 1:
		ct = pdf.DCTColorTransformYCbCr
	}
	rc, err := pdf.FilterDCT{ColorTransform: ct}.Decode(pdf.V1_7, bytes.NewReader(jpg), membudget.New(1<<30))
	if err != nil {
		return nil, nil, err
	}
	defer rc.Close()
	ref, err = io.ReadAll(rc)
	return jpg, ref, err
}

// samples returns the stripe pattern as interleaved samples.
func (r row) samples() []byte {
	pix := make([]byte, 0, w*h*r.comps)
	for y := range h {
		if r.comps == 4 {
			c := cmykStripes[y*len(cmykStripes)/h]
			for range w {
				pix = append(pix, c[:]...)
			}
		} else {
			c := rgbStripes[y*len(rgbStripes)/h]
			for range w {
				pix = append(pix, c.R, c.G, c.B)
			}
		}
	}
	return pix
}

// segments calls fn for each marker segment before the scan data,
// with the offset of the marker and the offset just after the segment.
func segments(jpg []byte, fn func(marker byte, start, end int)) {
	for pos := 2; pos+4 <= len(jpg) && jpg[pos] == 0xFF; {
		marker := jpg[pos+1]
		end := pos + 2 + int(binary.BigEndian.Uint16(jpg[pos+2:]))
		if end > len(jpg) {
			return
		}
		fn(marker, pos, end)
		if marker == 0xDA {
			return
		}
		pos = end
	}
}

// stripAPP14 removes the Adobe APP14 segment.
func stripAPP14(jpg []byte) []byte {
	var out []byte
	out = append(out, jpg[:2]...)
	last := 2
	segments(jpg, func(marker byte, start, end int) {
		if marker == 0xEE {
			out = append(out, jpg[last:start]...)
			last = end
		}
	})
	return append(out, jpg[last:]...)
}

// insertJFIF adds a JFIF segment directly after the SOI marker.
func insertJFIF(jpg []byte) []byte {
	jfif := []byte{0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0,
		1, 1, 0, 0, 1, 0, 1, 0, 0}
	out := append([]byte{}, jpg[:2]...)
	out = append(out, jfif...)
	return append(out, jpg[2:]...)
}

// setComponentIDs rewrites the component IDs in the SOF0 and SOS segments.
func setComponentIDs(jpg []byte, ids []byte) []byte {
	out := append([]byte{}, jpg...)
	segments(out, func(marker byte, start, end int) {
		switch marker {
		case 0xC0: // marker(2) length(2) precision(1) height(2) width(2) count(1) triples
			for i, id := range ids {
				out[start+10+3*i] = id
			}
		case 0xDA: // marker(2) length(2) count(1) pairs
			for i, id := range ids {
				out[start+2+3+2*i] = id
			}
		}
	})
	return out
}

// rawJPEGImage embeds pre-encoded JPEG bytes as a PDF image XObject.
type rawJPEGImage struct {
	data          []byte
	width, height int
	colorSpace    pdf.Name
	entry         int // /ColorTransform value, or absent
}

func (img *rawJPEGImage) Subtype() pdf.Name {
	return "Image"
}

func (img *rawJPEGImage) ResourceName() pdf.Name {
	return ""
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
	if img.entry != absent {
		dict["DecodeParms"] = pdf.Dict{
			"ColorTransform": pdf.Integer(img.entry),
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
