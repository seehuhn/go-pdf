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

package extract

import (
	"maps"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestXObjectImageMask checks that a dangling, null or wrong-type ImageMask
// entry reads as an ordinary image, and that ImageMask true gives an image
// mask.
func TestXObjectImageMask(t *testing.T) {
	extract := func(t *testing.T, extras pdf.Dict) any {
		t.Helper()
		w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
		dict := pdf.Dict{
			"Type":             pdf.Name("XObject"),
			"Subtype":          pdf.Name("Image"),
			"Width":            pdf.Integer(1),
			"Height":           pdf.Integer(1),
			"ColorSpace":       pdf.Name("DeviceGray"),
			"BitsPerComponent": pdf.Integer(8),
		}
		maps.Copy(dict, extras)
		ref := w.Alloc()
		body, err := w.OpenStream(ref, dict)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := body.Write([]byte{0}); err != nil {
			t.Fatal(err)
		}
		if err := body.Close(); err != nil {
			t.Fatal(err)
		}
		x, err := XObject(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
		if err != nil {
			t.Fatal(err)
		}
		return x
	}

	for name, obj := range map[string]pdf.Object{
		"dangling":   pdf.NewReference(99, 0),
		"null":       nil,
		"wrong type": pdf.Integer(1),
		"false":      pdf.Boolean(false),
	} {
		if got := extract(t, pdf.Dict{"ImageMask": obj}); got == nil {
			t.Errorf("%s: no image", name)
		} else if _, ok := got.(*image.Dict); !ok {
			t.Errorf("%s: got %T, want *image.Dict", name, got)
		}
	}

	got := extract(t, pdf.Dict{
		"ImageMask":        pdf.Boolean(true),
		"ColorSpace":       nil,
		"BitsPerComponent": pdf.Integer(1),
	})
	if _, ok := got.(*image.Mask); !ok {
		t.Errorf("got %T, want *image.Mask", got)
	}
}
