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

package image

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// testPalette is an indexed colour space with four distinct entries.
var testPalette = mustIndexed(
	color.DeviceRGB{1, 0, 0},
	color.DeviceRGB{0, 1, 0},
	color.DeviceRGB{0, 0, 1},
	color.DeviceRGB{0.5, 0.5, 0.5},
)

func mustIndexed(colors ...color.Color) color.Space {
	cs, err := color.Indexed(colors)
	if err != nil {
		panic(err)
	}
	return cs
}

// embedIndexed writes im to a fresh PDF file and reads the image dictionary
// back out again.
func embedIndexed(t *testing.T, v pdf.Version, im *Indexed) (*Dict, error) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(w)

	ref, err := rm.Embed(im)
	if err != nil {
		return nil, err
	}
	if err := rm.Close(); err != nil {
		return nil, err
	}

	x := pdf.NewExtractor(w)
	return ExtractDict(pdf.CursorAt(x, nil), ref, false)
}

// TestIndexedRoundTrip checks the guarantee that matters to a caller: the
// image a reader gets back describes the image which was written, and the
// samples are the palette positions they started as.  How the stream is
// compressed is not part of that promise, so the test says nothing about it.
func TestIndexedRoundTrip(t *testing.T) {
	im := NewIndexed(5, 3, testPalette)
	for i := range im.Pix {
		im.Pix[i] = uint8(i % 4)
	}

	dict, err := embedIndexed(t, pdf.V1_7, im)
	if err != nil {
		t.Fatal(err)
	}

	if dict.Width != im.Width || dict.Height != im.Height {
		t.Errorf("got %dx%d, want %dx%d",
			dict.Width, dict.Height, im.Width, im.Height)
	}
	// an indexed sample is a palette position, and the palette here has four
	// entries, so 8 bits per component is what Indexed writes
	if dict.BitsPerComponent != 8 {
		t.Errorf("got BitsPerComponent %d, want 8", dict.BitsPerComponent)
	}
	if got := dict.ColorSpace.Family(); got != color.FamilyIndexed {
		t.Errorf("got colour space family %q, want %q", got, color.FamilyIndexed)
	}

	pix, err := dict.Data.Pixels()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pix, im.Pix) {
		t.Errorf("samples not recovered:\n got %v\nwant %v", pix, im.Pix)
	}
}

// TestIndexedInvalid checks that Embed refuses input which would produce an
// invalid PDF file, rather than writing it out.
func TestIndexedInvalid(t *testing.T) {
	t.Run("wrong colour space", func(t *testing.T) {
		im := NewIndexed(2, 2, color.SpaceDeviceRGB)
		if _, err := embedIndexed(t, pdf.V1_7, im); err == nil {
			t.Error("no error for a non-indexed colour space")
		}
	})

	t.Run("index outside palette", func(t *testing.T) {
		im := NewIndexed(2, 2, testPalette)
		im.Pix[2] = 4 // the palette holds entries 0 to 3
		if _, err := embedIndexed(t, pdf.V1_7, im); err == nil {
			t.Error("no error for an out-of-range palette index")
		}
	})
}

// TestIndexedXObject checks the [graphics.XObject] methods.  Indexed has no
// Name field, so it never claims a resource key of its own.
func TestIndexedXObject(t *testing.T) {
	im := NewIndexed(2, 2, testPalette)
	if got := im.ResourceName(); got != "" {
		t.Errorf("got %q, want the empty name", got)
	}
	if got := im.Subtype(); got != "Image" {
		t.Errorf("got subtype %q, want %q", got, "Image")
	}

	b := im.Bounds()
	if b.XMax != 2 || b.YMax != 2 {
		t.Errorf("got bounds %v, want 2x2", b)
	}
}
