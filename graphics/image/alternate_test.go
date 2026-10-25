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
	"maps"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// extractAlternateWith reads an alternate image dictionary.  The image is a
// 1x1 gray image whose dictionary contains the entries in imageExtras, and
// the alternate dictionary contains the entries in extras.
func extractAlternateWith(t *testing.T, imageExtras, extras pdf.Dict) *Alternate {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	imgDict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(1),
		"Height":           pdf.Integer(1),
		"ColorSpace":       pdf.Name("DeviceGray"),
		"BitsPerComponent": pdf.Integer(8),
	}
	maps.Copy(imgDict, imageExtras)
	imgRef := w.Alloc()
	body, err := w.OpenStream(imgRef, imgDict)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := body.Write([]byte{0}); err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}

	dict := pdf.Dict{"Image": imgRef}
	maps.Copy(dict, extras)
	alt, err := ExtractAlternate(pdf.CursorAt(pdf.NewExtractor(w), nil), dict, true)
	if err != nil {
		t.Fatal(err)
	}
	return alt
}

// TestExtractAlternateDefaultForPrinting checks that a dangling, null or
// wrong-type DefaultForPrinting entry reads as false.
func TestExtractAlternateDefaultForPrinting(t *testing.T) {
	for name, obj := range map[string]pdf.Object{
		"dangling":   pdf.NewReference(99, 0),
		"null":       nil,
		"wrong type": pdf.Integer(1),
		"false":      pdf.Boolean(false),
	} {
		alt := extractAlternateWith(t, nil, pdf.Dict{"DefaultForPrinting": obj})
		if alt.DefaultForPrinting {
			t.Errorf("%s: DefaultForPrinting is true", name)
		}
	}

	alt := extractAlternateWith(t, nil, pdf.Dict{"DefaultForPrinting": pdf.Boolean(true)})
	if !alt.DefaultForPrinting {
		t.Error("DefaultForPrinting not read")
	}
}

// TestExtractAlternateImageMask checks that a dangling, null or wrong-type
// ImageMask entry in the image dictionary reads as an ordinary image, and
// that ImageMask true gives an image mask.
func TestExtractAlternateImageMask(t *testing.T) {
	for name, obj := range map[string]pdf.Object{
		"dangling":   pdf.NewReference(99, 0),
		"null":       nil,
		"wrong type": pdf.Integer(1),
		"false":      pdf.Boolean(false),
	} {
		alt := extractAlternateWith(t, pdf.Dict{"ImageMask": obj}, nil)
		if _, ok := alt.Image.(*Dict); !ok {
			t.Errorf("%s: got image of type %T, want *Dict", name, alt.Image)
		}
	}

	alt := extractAlternateWith(t, pdf.Dict{
		"ImageMask":        pdf.Boolean(true),
		"ColorSpace":       nil,
		"BitsPerComponent": pdf.Integer(1),
	}, nil)
	if _, ok := alt.Image.(*Mask); !ok {
		t.Errorf("got image of type %T, want *Mask", alt.Image)
	}
}
