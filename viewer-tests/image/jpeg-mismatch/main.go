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

// Command jpeg-mismatch generates a one-page PDF whose image XObject
// declares /Width 32 /Height 32 in the dictionary but carries a 64×64
// baseline JPEG in the stream.  Open the resulting test.pdf in different
// PDF viewers to see how each handles the mismatch.
//
// The JPEG is a four-quadrant pattern (red, green, blue, yellow) with a
// small dark square in the outer corner of each quadrant.  A viewer that
// trusts the dictionary tends to render only the top-left 32×32 region
// (just the red quadrant with one corner mark); a viewer that trusts the
// JPEG header renders all four quadrants.  The corner marks make any
// cropping or partial rendering visible at a glance.
package main

import (
	"bytes"
	"fmt"
	stdimg "image"
	"image/color"
	"image/jpeg"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/graphics"
)

const (
	pageSize  = 200.0
	imageSize = 160.0
	margin    = (pageSize - imageSize) / 2
)

const (
	dictDim = 32
	jpegDim = 64
)

func main() {
	if err := run("test.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(filename string) error {
	paper := &pdf.Rectangle{URx: pageSize, URy: pageSize}
	page, err := document.CreateSinglePage(filename, paper, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	data, err := makeJPEG()
	if err != nil {
		return err
	}
	img := &mismatchImage{data: data}

	page.PushGraphicsState()
	page.Transform(matrix.Matrix{imageSize, 0, 0, imageSize, margin, margin})
	page.DrawXObject(img)
	page.PopGraphicsState()

	return page.Close()
}

// makeJPEG returns a baseline 64×64 JPEG with four coloured quadrants
// and a dark corner marker in each, encoded by the stdlib JPEG encoder
// (RGB input, YCbCr 4:2:0 in the wire format).
func makeJPEG() ([]byte, error) {
	src := stdimg.NewRGBA(stdimg.Rect(0, 0, jpegDim, jpegDim))

	half := jpegDim / 2
	mark := color.RGBA{0x20, 0x20, 0x20, 0xff}
	quads := []struct {
		ox, oy int
		bg     color.RGBA
		mx, my int
	}{
		// top-left red, mark in TL corner
		{0, 0, color.RGBA{0xc8, 0x44, 0x44, 0xff}, 4, 4},
		// top-right green, mark in TR corner
		{half, 0, color.RGBA{0x44, 0xa4, 0x44, 0xff}, half - 4, 4},
		// bottom-left blue, mark in BL corner
		{0, half, color.RGBA{0x44, 0x70, 0xc4, 0xff}, 4, half - 4},
		// bottom-right yellow, mark in BR corner
		{half, half, color.RGBA{0xc8, 0xb0, 0x44, 0xff}, half - 4, half - 4},
	}

	for _, q := range quads {
		for y := range half {
			for x := range half {
				src.SetRGBA(q.ox+x, q.oy+y, q.bg)
			}
		}
		// 4×4 marker centred on (mx, my)
		for dy := -2; dy < 2; dy++ {
			for dx := -2; dx < 2; dx++ {
				src.SetRGBA(q.ox+q.mx+dx, q.oy+q.my+dy, mark)
			}
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// mismatchImage is an Image XObject whose dictionary lies about the
// pixel dimensions: /Width and /Height claim 32×32, but the JPEG SOF
// inside the stream declares 64×64.
type mismatchImage struct {
	data []byte
}

var _ graphics.XObject = (*mismatchImage)(nil)

func (m *mismatchImage) Subtype() pdf.Name      { return "Image" }
func (m *mismatchImage) ResourceName() pdf.Name { return "" }

func (m *mismatchImage) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(dictDim),
		"Height":           pdf.Integer(dictDim),
		"ColorSpace":       pdf.Name("DeviceRGB"),
		"BitsPerComponent": pdf.Integer(8),
		"Filter":           pdf.Name("DCTDecode"),
	}
	ref := rm.Alloc()
	stm, err := rm.Out().OpenStream(ref, dict)
	if err != nil {
		return nil, err
	}
	if _, err := stm.Write(m.data); err != nil {
		return nil, err
	}
	if err := stm.Close(); err != nil {
		return nil, err
	}
	return ref, nil
}
