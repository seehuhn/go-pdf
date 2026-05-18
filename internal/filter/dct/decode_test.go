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

package dct

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math"
	"os"
	"testing"
)

func TestDecodeRGB(t *testing.T) {
	const w, h = 16, 16

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 16),
				G: uint8(y * 16),
				B: uint8((x + y) * 8),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	jpegBytes := buf.Bytes()

	// decode using our function
	rc, err := Decode(bytes.NewReader(jpegBytes), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != w*h*3 {
		t.Fatalf("got %d bytes, want %d", len(data), w*h*3)
	}

	// decode using standard library for reference
	ref, err := jpeg.Decode(bytes.NewReader(jpegBytes))
	if err != nil {
		t.Fatal(err)
	}

	// our output must match the reference exactly
	i := 0
	for y := range h {
		for x := range w {
			r, g, b, _ := ref.At(x, y).RGBA()
			wantR := uint8(r >> 8)
			wantG := uint8(g >> 8)
			wantB := uint8(b >> 8)
			if data[i] != wantR || data[i+1] != wantG || data[i+2] != wantB {
				t.Errorf("pixel (%d,%d): got (%d,%d,%d), want (%d,%d,%d)",
					x, y, data[i], data[i+1], data[i+2], wantR, wantG, wantB)
			}
			i += 3
		}
	}
}

func TestDecodeGrayscale(t *testing.T) {
	const w, h = 16, 16

	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetGray(x, y, color.Gray{Y: uint8((x + y) * 8)})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	jpegBytes := buf.Bytes()

	rc, err := Decode(bytes.NewReader(jpegBytes), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != w*h {
		t.Fatalf("got %d bytes, want %d", len(data), w*h)
	}

	// decode using standard library for reference
	ref, err := jpeg.Decode(bytes.NewReader(jpegBytes))
	if err != nil {
		t.Fatal(err)
	}

	i := 0
	for y := range h {
		for x := range w {
			r, _, _, _ := ref.At(x, y).RGBA()
			want := uint8(r >> 8)
			if !closeEnough(data[i], want) {
				t.Errorf("pixel (%d,%d): got %d, want %d", x, y, data[i], want)
			}
			i++
		}
	}
}

func TestDecodeCMYK(t *testing.T) {
	jpegBytes, err := os.ReadFile("testdata/cmyk.jpg")
	if err != nil {
		t.Fatal(err)
	}

	rc, err := Decode(bytes.NewReader(jpegBytes), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}

	// decode using standard library for reference
	ref, err := jpeg.Decode(bytes.NewReader(jpegBytes))
	if err != nil {
		t.Fatal(err)
	}
	cmykImg, ok := ref.(*image.CMYK)
	if !ok {
		t.Fatalf("expected *image.CMYK, got %T", ref)
	}

	bounds := cmykImg.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if len(data) != w*h*4 {
		t.Fatalf("got %d bytes, want %d", len(data), w*h*4)
	}

	// verify pixel values match the reference.
	// Go's image.CMYK uses Adobe convention (0 = full ink),
	// but our Decode returns PDF convention (0 = no ink), so we invert.
	i := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			off := cmykImg.PixOffset(x, y)
			wantC := 255 - cmykImg.Pix[off]
			wantM := 255 - cmykImg.Pix[off+1]
			wantY := 255 - cmykImg.Pix[off+2]
			wantK := 255 - cmykImg.Pix[off+3]
			if data[i] != wantC || data[i+1] != wantM || data[i+2] != wantY || data[i+3] != wantK {
				t.Errorf("pixel (%d,%d): got (%d,%d,%d,%d), want (%d,%d,%d,%d)",
					x, y, data[i], data[i+1], data[i+2], data[i+3],
					wantC, wantM, wantY, wantK)
			}
			i += 4
		}
	}
}

func TestDecodeColorTransform(t *testing.T) {
	// create a simple RGB image and encode it as JPEG
	const w, h = 8, 8
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	jpegBytes := buf.Bytes()

	// decode with default (nil) ColorTransform — should give RGB
	rcDefault, err := Decode(bytes.NewReader(jpegBytes), nil)
	if err != nil {
		t.Fatal(err)
	}
	dataDefault, err := io.ReadAll(rcDefault)
	rcDefault.Close()
	if err != nil {
		t.Fatal(err)
	}

	// decode with ColorTransform=1 (YCbCr→RGB) — same as default
	ct1 := 1
	rcCT1, err := Decode(bytes.NewReader(jpegBytes), &ct1)
	if err != nil {
		t.Fatal(err)
	}
	dataCT1, err := io.ReadAll(rcCT1)
	rcCT1.Close()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(dataDefault, dataCT1) {
		t.Error("ColorTransform=1 should produce the same result as default")
	}

	// decode with ColorTransform=0 (no transform) — raw YCbCr values
	ct0 := 0
	rcCT0, err := Decode(bytes.NewReader(jpegBytes), &ct0)
	if err != nil {
		t.Fatal(err)
	}
	dataCT0, err := io.ReadAll(rcCT0)
	rcCT0.Close()
	if err != nil {
		t.Fatal(err)
	}

	// with ColorTransform=0, the output should differ from the default
	// because the YCbCr data is passed through without conversion
	if bytes.Equal(dataDefault, dataCT0) {
		t.Error("ColorTransform=0 should produce different output than default")
	}
}

func closeEnough(a, b uint8) bool {
	return math.Abs(float64(a)-float64(b)) <= 1
}

// TestDecodeProgressiveBudget verifies that a progressive JPEG whose
// SOF dimensions fit the per-image pixel/byte caps but would push the
// internal coefficient buffer above the budget is rejected before any
// large allocation happens.
func TestDecodeProgressiveBudget(t *testing.T) {
	// build a minimal SOI + SOF2 + SOS sequence; entropy data is not
	// required because the cap fires at the progCoeffs allocation in
	// scan.go before any Huffman decoding
	build := func(w, h uint16) []byte {
		hi := func(v uint16) byte { return byte(v >> 8) }
		lo := func(v uint16) byte { return byte(v) }
		return []byte{
			0xFF, 0xD8, // SOI
			0xFF, 0xC2, // SOF2 (progressive)
			0x00, 0x0B, // length = 11
			0x08,         // precision = 8
			hi(h), lo(h), // height
			hi(w), lo(w), // width
			0x01,       // nComp = 1
			0x01,       // component id
			0x11,       // sampling h=1 v=1
			0x00,       // quant table 0
			0xFF, 0xDA, // SOS
			0x00, 0x08, // length = 8
			0x01, // nComp in scan = 1
			0x01, // component selector
			0x00, // td=0, ta=0
			0x00, // Ss = 0 (DC-only scan, valid for progressive)
			0x00, // Se = 0
			0x00, // Ah = 0, Al = 0
		}
	}

	// dimensions chosen to pass ImagePixelsExceedLimit (128 Mpx) and
	// ImageBytesExceedLimit (256 MiB) at SOF parse time, but to require
	// > 1 Mi progressive blocks (= 256 MiB at 256 B/block)
	payload := build(10000, 10000)
	rc, err := Decode(bytes.NewReader(payload), nil)
	if err == nil {
		_, err = io.ReadAll(rc)
		rc.Close()
	}
	if err == nil {
		t.Fatal("expected error for oversize progressive scan, got nil")
	}
}

// TestDecodeOversizeSOF verifies that a JPEG declaring dimensions whose
// product exceeds streamlimits.MaxImageBytes is rejected by the SOF
// parser before any large allocation happens.
func TestDecodeOversizeSOF(t *testing.T) {
	// build a minimal SOI + SOF0 + EOI sequence claiming the given size
	build := func(nComp int, w, h uint16) []byte {
		hi := func(v uint16) byte { return byte(v >> 8) }
		lo := func(v uint16) byte { return byte(v) }
		sof := []byte{
			0xFF, 0xC0, // SOF0
			0x00, byte(8 + 3*nComp), // length
			0x08,         // precision = 8
			hi(h), lo(h), // height
			hi(w), lo(w), // width
			byte(nComp), // number of components
		}
		// per-component spec: id, sampling (0x11 = 1x1), quant table index
		for i := range nComp {
			sof = append(sof, byte(i+1), 0x11, 0x00)
		}
		out := []byte{0xFF, 0xD8} // SOI
		out = append(out, sof...)
		out = append(out, 0xFF, 0xD9) // EOI
		return out
	}

	for _, tc := range []struct {
		name  string
		nComp int
	}{
		{"grayscale", 1},
		{"ycbcr", 3},
		{"cmyk", 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := build(tc.nComp, 65535, 65535)
			rc, err := Decode(bytes.NewReader(payload), nil)
			if err == nil {
				_, err = io.ReadAll(rc)
				rc.Close()
			}
			if err == nil {
				t.Fatal("expected error for oversize SOF, got nil")
			}
		})
	}
}
