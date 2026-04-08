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

import "testing"

func TestCombineOR(t *testing.T) {
	dst := New(8, 1)
	dst.Pix[0] = 0xF0 // 11110000

	src := New(8, 1)
	src.Pix[0] = 0x0F // 00001111

	dst.Combine(src, 0, 0, CombOpOR)
	if dst.Pix[0] != 0xFF {
		t.Fatalf("OR: got 0x%02x, want 0xFF", dst.Pix[0])
	}
}

func TestCombineAND(t *testing.T) {
	dst := New(8, 1)
	dst.Pix[0] = 0xF0

	src := New(8, 1)
	src.Pix[0] = 0x3C // 00111100

	dst.Combine(src, 0, 0, CombOpAND)
	if dst.Pix[0] != 0x30 {
		t.Fatalf("AND: got 0x%02x, want 0x30", dst.Pix[0])
	}
}

func TestCombineXOR(t *testing.T) {
	dst := New(8, 1)
	dst.Pix[0] = 0xF0

	src := New(8, 1)
	src.Pix[0] = 0xFF

	dst.Combine(src, 0, 0, CombOpXOR)
	if dst.Pix[0] != 0x0F {
		t.Fatalf("XOR: got 0x%02x, want 0x0F", dst.Pix[0])
	}
}

func TestCombineXNOR(t *testing.T) {
	dst := New(8, 1)
	dst.Pix[0] = 0xF0

	src := New(8, 1)
	src.Pix[0] = 0xFF

	dst.Combine(src, 0, 0, CombOpXNOR)
	if dst.Pix[0] != 0xF0 {
		t.Fatalf("XNOR: got 0x%02x, want 0xF0", dst.Pix[0])
	}
}

func TestCombineReplace(t *testing.T) {
	dst := New(8, 1)
	dst.Pix[0] = 0xF0

	src := New(8, 1)
	src.Pix[0] = 0x55

	dst.Combine(src, 0, 0, CombOpReplace)
	if dst.Pix[0] != 0x55 {
		t.Fatalf("REPLACE: got 0x%02x, want 0x55", dst.Pix[0])
	}
}

func TestCombineOffset(t *testing.T) {
	dst := New(16, 8)
	src := New(8, 4)
	for y := range 4 {
		for x := range 8 {
			src.SetPixel(x, y, true)
		}
	}

	dst.Combine(src, 4, 2, CombOpOR)

	// check a pixel inside the placed region
	if !dst.GetPixel(4, 2) {
		t.Fatal("pixel (4,2) should be black")
	}
	if !dst.GetPixel(11, 5) {
		t.Fatal("pixel (11,5) should be black")
	}
	// check pixels outside the placed region
	if dst.GetPixel(3, 2) {
		t.Fatal("pixel (3,2) should be white")
	}
	if dst.GetPixel(12, 2) {
		t.Fatal("pixel (12,2) should be white")
	}
	if dst.GetPixel(4, 1) {
		t.Fatal("pixel (4,1) should be white")
	}
}

func TestCombineClipping(t *testing.T) {
	dst := New(8, 8)
	src := New(8, 8)
	for i := range src.Pix {
		src.Pix[i] = 0xFF
	}

	// place src partially outside dst
	dst.Combine(src, 4, 4, CombOpOR)

	// pixels inside overlap should be set
	if !dst.GetPixel(4, 4) {
		t.Fatal("pixel (4,4) should be black")
	}
	if !dst.GetPixel(7, 7) {
		t.Fatal("pixel (7,7) should be black")
	}
	// pixels outside overlap should remain white
	if dst.GetPixel(3, 4) {
		t.Fatal("pixel (3,4) should be white")
	}
}

func TestCombineNil(t *testing.T) {
	dst := New(8, 8)
	dst.Combine(nil, 0, 0, CombOpOR) // should not panic
}
