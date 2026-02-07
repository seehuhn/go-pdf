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
	rc, err := Decode(bytes.NewReader(jpegBytes))
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

	rc, err := Decode(bytes.NewReader(jpegBytes))
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

	rc, err := Decode(bytes.NewReader(jpegBytes))
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

	// verify pixel values match the reference
	i := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			off := cmykImg.PixOffset(x, y)
			wantC := cmykImg.Pix[off]
			wantM := cmykImg.Pix[off+1]
			wantY := cmykImg.Pix[off+2]
			wantK := cmykImg.Pix[off+3]
			if data[i] != wantC || data[i+1] != wantM || data[i+2] != wantY || data[i+3] != wantK {
				t.Errorf("pixel (%d,%d): got (%d,%d,%d,%d), want (%d,%d,%d,%d)",
					x, y, data[i], data[i+1], data[i+2], data[i+3],
					wantC, wantM, wantY, wantK)
			}
			i += 4
		}
	}
}

func closeEnough(a, b uint8) bool {
	return math.Abs(float64(a)-float64(b)) <= 1
}
