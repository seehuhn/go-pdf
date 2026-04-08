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

package bitmap

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	bm := New(16, 8)
	if bm.Width() != 16 || bm.Height() != 8 {
		t.Fatalf("got %dx%d, want 16x8", bm.Width(), bm.Height())
	}
	if bm.Stride != 2 {
		t.Fatalf("stride = %d, want 2", bm.Stride)
	}
	if len(bm.Pix) != 16 {
		t.Fatalf("len(Pix) = %d, want 16", len(bm.Pix))
	}
}

func TestNewOddWidth(t *testing.T) {
	bm := New(10, 1)
	if bm.Stride != 2 {
		t.Fatalf("stride = %d, want 2", bm.Stride)
	}
}

func TestGetSetPixel(t *testing.T) {
	bm := New(16, 16)

	// initially all white
	for y := range 16 {
		for x := range 16 {
			if bm.GetPixel(x, y) {
				t.Fatalf("pixel (%d,%d) should be white", x, y)
			}
		}
	}

	// set some pixels
	bm.SetPixel(0, 0, true)
	bm.SetPixel(7, 0, true)
	bm.SetPixel(8, 0, true)
	bm.SetPixel(15, 0, true)

	if !bm.GetPixel(0, 0) {
		t.Fatal("pixel (0,0) should be black")
	}
	if !bm.GetPixel(7, 0) {
		t.Fatal("pixel (7,0) should be black")
	}
	if !bm.GetPixel(8, 0) {
		t.Fatal("pixel (8,0) should be black")
	}
	if !bm.GetPixel(15, 0) {
		t.Fatal("pixel (15,0) should be black")
	}

	// verify byte values (MSB-first)
	if bm.Pix[0] != 0x81 {
		t.Fatalf("Pix[0] = 0x%02x, want 0x81", bm.Pix[0])
	}
	if bm.Pix[1] != 0x81 {
		t.Fatalf("Pix[1] = 0x%02x, want 0x81", bm.Pix[1])
	}

	// clear a pixel
	bm.SetPixel(0, 0, false)
	if bm.GetPixel(0, 0) {
		t.Fatal("pixel (0,0) should be white after clearing")
	}
}

func TestOutOfBounds(t *testing.T) {
	bm := New(8, 8)
	bm.SetPixel(-1, 0, true) // no-op
	bm.SetPixel(8, 0, true)  // no-op
	bm.SetPixel(0, -1, true) // no-op
	bm.SetPixel(0, 8, true)  // no-op
	if bm.GetPixel(-1, 0) {
		t.Fatal("out of bounds should return false")
	}
	if bm.GetPixel(8, 0) {
		t.Fatal("out of bounds should return false")
	}
}

func TestSubImageAligned(t *testing.T) {
	bm := New(16, 8)
	bm.SetPixel(8, 0, true)
	bm.SetPixel(15, 7, true)

	sub := bm.SubImage(image.Rect(8, 0, 16, 8))
	if sub.Width() != 8 || sub.Height() != 8 {
		t.Fatalf("sub size = %dx%d, want 8x8", sub.Width(), sub.Height())
	}
	if !sub.GetPixel(8, 0) {
		t.Fatal("sub pixel (8,0) should be black")
	}
	if !sub.GetPixel(15, 7) {
		t.Fatal("sub pixel (15,7) should be black")
	}
	if sub.GetPixel(9, 0) {
		t.Fatal("sub pixel (9,0) should be white")
	}
}

func TestSubImageUnaligned(t *testing.T) {
	bm := New(16, 8)
	bm.SetPixel(5, 3, true)

	sub := bm.SubImage(image.Rect(4, 2, 12, 6))
	if sub.Width() != 8 || sub.Height() != 4 {
		t.Fatalf("sub size = %dx%d, want 8x4", sub.Width(), sub.Height())
	}
	if !sub.GetPixel(5, 3) {
		t.Fatal("sub pixel (5,3) should be black")
	}
	if sub.GetPixel(4, 2) {
		t.Fatal("sub pixel (4,2) should be white")
	}
}

func TestImageInterface(t *testing.T) {
	bm := New(8, 8)
	bm.SetPixel(0, 0, true)

	var img image.Image = bm
	if img.Bounds() != image.Rect(0, 0, 8, 8) {
		t.Fatalf("bounds = %v, want (0,0)-(8,8)", img.Bounds())
	}

	c := img.At(0, 0)
	if c != color.Black {
		t.Fatalf("At(0,0) = %v, want Black", c)
	}
	c = img.At(1, 0)
	if c != color.White {
		t.Fatalf("At(1,0) = %v, want White", c)
	}
}

func TestSetColor(t *testing.T) {
	bm := New(8, 8)
	bm.Set(0, 0, color.Black)
	bm.Set(1, 0, color.White)
	bm.Set(2, 0, color.RGBA{R: 10, G: 10, B: 10, A: 255}) // dark -> black

	if !bm.GetPixel(0, 0) {
		t.Fatal("(0,0) should be black")
	}
	if bm.GetPixel(1, 0) {
		t.Fatal("(1,0) should be white")
	}
	if !bm.GetPixel(2, 0) {
		t.Fatal("(2,0) should be black (dark color)")
	}
}

func TestReadBMP(t *testing.T) {
	testdataDir := "../../internal/filter/jbig2/docs/testdata/decode"
	pattern := filepath.Join(testdataDir, "*.bmp")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no BMP test files found")
	}

	for _, path := range files {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			bm, err := ReadBMP(f)
			if err != nil {
				t.Fatal(err)
			}
			if bm.Width() <= 0 || bm.Height() <= 0 {
				t.Fatalf("invalid dimensions %dx%d", bm.Width(), bm.Height())
			}
		})
	}
}
