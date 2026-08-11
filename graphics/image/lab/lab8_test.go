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

package lab

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	pdfcolor "seehuhn.de/go/pdf/graphics/color"
	pdfimage "seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestNewLab8 checks the documented shape of the pixel buffer: three uint8
// values per pixel, one each for L, a and b.
func TestNewLab8(t *testing.T) {
	im := NewLab8(4, 3)

	if im.Width != 4 || im.Height != 3 {
		t.Errorf("got %dx%d, want 4x3", im.Width, im.Height)
	}
	if got, want := len(im.PixData), 4*3*3; got != want {
		t.Errorf("got %d pixel bytes, want %d", got, want)
	}
	if got := im.Subtype(); got != "Image" {
		t.Errorf("got subtype %q, want %q", got, "Image")
	}
	// Lab8 has no Name field, so it never claims a resource key of its own
	if got := im.ResourceName(); got != "" {
		t.Errorf("got resource name %q, want the empty name", got)
	}
}

// TestLab8RoundTrip checks the guarantee a caller depends on: the image a
// reader gets back is the one which was written.  The samples must survive
// unchanged, and the dictionary must describe them as Lab values over the
// full L, a and b ranges.
//
// The Decode check is a check on what a reader sees, not on what was written:
// these are the default values for a Lab image at 8 bits, so a reader arrives
// at them whether or not the array is present in the file.
//
// The test says nothing about how the stream is compressed; that is free to
// change without breaking any promise made to the caller.
func TestLab8RoundTrip(t *testing.T) {
	im := NewLab8(4, 3)
	for i := range im.PixData {
		im.PixData[i] = uint8(i * 11)
	}

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	ref, err := rm.Embed(im)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	dict, err := pdfimage.ExtractDict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}

	if dict.Width != im.Width || dict.Height != im.Height {
		t.Errorf("got %dx%d, want %dx%d",
			dict.Width, dict.Height, im.Width, im.Height)
	}
	if dict.BitsPerComponent != 8 {
		t.Errorf("got BitsPerComponent %d, want 8", dict.BitsPerComponent)
	}
	if got := dict.ColorSpace.Family(); got != pdfcolor.FamilyLab {
		t.Errorf("got colour space family %q, want %q", got, pdfcolor.FamilyLab)
	}

	// L covers 0 to 100, a and b the Lab space's default range
	wantDecode := []float64{0, 100, -100, 100, -100, 100}
	if len(dict.Decode) != len(wantDecode) {
		t.Errorf("got Decode %v, want %v", dict.Decode, wantDecode)
	} else {
		for i, v := range wantDecode {
			if dict.Decode[i] != v {
				t.Errorf("got Decode %v, want %v", dict.Decode, wantDecode)
				break
			}
		}
	}

	pix, err := dict.Data.Pixels()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pix, im.PixData) {
		t.Errorf("samples not recovered:\n got %v\nwant %v", pix, im.PixData)
	}
}
