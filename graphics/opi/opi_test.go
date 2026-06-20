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

package opi

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func spec(name string) *file.Specification {
	return &file.Specification{FileName: name, AFRelationship: file.RelationshipUnspecified}
}

func ptr[T any](v T) *T { return &v }

var testCases = []struct {
	name string
	dict Dict
}{
	{
		name: "v13 minimal",
		dict: &V13{
			F:        spec("image.tif"),
			Size:     [2]int{640, 480},
			CropRect: [4]int{0, 0, 640, 480},
			Position: [8]float64{0, 0, 0, 480, 640, 480, 640, 0},
		},
	},
	{
		name: "v13 full",
		dict: &V13{
			F:            spec("photo.tif"),
			ID:           pdf.String("image-id-42"),
			Comments:     "handle with care",
			Size:         [2]int{1024, 768},
			CropRect:     [4]int{10, 10, 1000, 700},
			CropFixed:    &[4]float64{10.5, 10.5, 1000.5, 700.5},
			Position:     [8]float64{0, 0, 0, 100, 100, 100, 100, 0},
			Resolution:   &[2]float64{300, 300},
			ColorType:    "Separation",
			Color:        &Color13{CMYK: [4]float64{0, 0, 0, 1}, Name: pdf.String("Black")},
			Tint:         ptr(0.5),
			Overprint:    true,
			ImageType:    &[2]int{3, 8},
			GrayMap:      []int{0, 65535, 32768},
			Transparency: ptr(false),
			Tags: []Tag13{
				{Num: 270, Text: "description"},
				{Num: 271, Text: "make"},
			},
		},
	},
	{
		name: "v20 minimal",
		dict: &V20{
			F: spec("proxy.tif"),
		},
	},
	{
		name: "v20 full, named inks",
		dict: &V20{
			F:         spec("proxy.tif"),
			MainImage: pdf.String("/vol/images/full.tif"),
			Tags: []Tag20{
				{Num: 270, Text: []string{"a description"}},
				{Num: 333, Text: []string{"first", "second"}},
			},
			Size:                    &[2]float64{800, 600},
			CropRect:                &[4]float64{0, 0, 800, 600},
			Overprint:               true,
			Inks:                    &Inks20{Name: "full_color"},
			IncludedImageDimensions: &[2]int{80, 60},
			IncludedImageQuality:    2,
		},
	},
	{
		name: "v20 monochrome inks",
		dict: &V20{
			F: spec("proxy.tif"),
			Inks: &Inks20{Monochrome: []Ink20Comp{
				{Name: pdf.String("PANTONE 123"), Tint: 1.0},
				{Name: pdf.String("PANTONE 456"), Tint: 0.5},
			}},
		},
	},
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripTest(t, pdf.V1_7, tc.dict)
		})
	}
}

func roundTripTest(t *testing.T, version pdf.Version, d1 Dict) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	obj, err := rm.Embed(d1)
	if pdf.IsWrongVersion(err) {
		t.Skip("version not supported")
	} else if err != nil {
		t.Fatalf("embed failed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	d2, err := Extract(pdf.CursorAt(x, nil), obj, true)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if !d1.Equal(d2) {
		t.Errorf("round trip failed:\n got %+v\nwant %+v", d2, d1)
	}
}

// TestReadInksDegenerate checks that an Inks array carrying no usable
// colourant pairs reads back as nil, so a read-write-read cycle is stable
// (Embed writes nothing for an empty Inks20, which must read back as nil).
func TestReadInksDegenerate(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)
	for _, obj := range []pdf.Object{pdf.Array{}, pdf.Array{pdf.Name("monochrome")}} {
		inks, err := readInks(pdf.CursorAt(x, nil), obj)
		if err != nil {
			t.Fatal(err)
		}
		if inks != nil {
			t.Errorf("readInks(%v) = %+v, want nil", obj, inks)
		}
	}
}

// TestReadTagTextDegenerate checks that a tag value that is neither a string
// nor a non-empty array reads back as nil, matching what Embed writes for an
// empty tag value.
func TestReadTagTextDegenerate(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)
	for _, obj := range []pdf.Object{pdf.Integer(42), pdf.Array{}} {
		text, err := readTagText(pdf.CursorAt(x, nil), obj)
		if err != nil {
			t.Fatal(err)
		}
		if text != nil {
			t.Errorf("readTagText(%v) = %#v, want nil", obj, text)
		}
	}
}

// TestV20WriteValidation checks that Embed rejects V20 dictionaries that
// violate the Size/CropRect togetherness rule or carry an invalid
// IncludedImageQuality.
func TestV20WriteValidation(t *testing.T) {
	bad := []*V20{
		{F: spec("p.tif"), Size: &[2]float64{1, 1}},
		{F: spec("p.tif"), CropRect: &[4]float64{0, 0, 1, 1}},
		{F: spec("p.tif"), IncludedImageQuality: 5},
	}
	for i, v := range bad {
		w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
		rm := pdf.NewResourceManager(w)
		if _, err := rm.Embed(v); err == nil {
			t.Errorf("case %d: expected error, got nil", i)
		}
	}
}

// TestV20ReadFix checks that reading a dictionary with only one of Size/CropRect
// or an out-of-range IncludedImageQuality snaps to a writable state, so the
// read-write-read cycle stays stable.
func TestV20ReadFix(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	fObj, err := rm.Embed(spec("proxy.tif"))
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	dict := pdf.Dict{
		"Type":                 pdf.Name("OPI"),
		"Version":              pdf.Number(2.0),
		"F":                    fObj,
		"Size":                 pdf.Array{pdf.Number(640), pdf.Number(480)},
		"IncludedImageQuality": pdf.Number(5),
	}
	x := pdf.NewExtractor(w)
	v, err := extractV20(pdf.CursorAt(x, nil), dict, true)
	if err != nil {
		t.Fatal(err)
	}
	if v.Size != nil || v.CropRect != nil {
		t.Errorf("lone Size not dropped: Size=%v CropRect=%v", v.Size, v.CropRect)
	}
	if v.IncludedImageQuality != 0 {
		t.Errorf("invalid IncludedImageQuality not dropped: %v", v.IncludedImageQuality)
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(pdf.V1_7, opt)
		if err := memfile.AddBlankPage(w); err != nil {
			continue
		}
		rm := pdf.NewResourceManager(w)
		obj, err := rm.Embed(tc.dict)
		if err != nil {
			continue
		}
		if err := rm.Close(); err != nil {
			continue
		}
		w.GetMeta().Trailer["Quir:E"] = obj
		if err := w.Close(); err != nil {
			continue
		}
		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing object")
		}
		x := pdf.NewExtractor(r)
		d, err := Extract(pdf.CursorAt(x, nil), obj, false)
		if err != nil {
			t.Skip("malformed OPI dictionary")
		}
		roundTripTest(t, pdf.GetVersion(r), d)
	})
}
