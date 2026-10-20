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
	"errors"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func spec(name string) *file.Specification {
	return &file.Specification{FileName: name, AFRelationship: file.RelationshipUnspecified}
}

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
			Comments:     pdf.Text("handle with care"),
			Size:         [2]int{1024, 768},
			CropRect:     [4]int{10, 10, 1000, 700},
			CropFixed:    &[4]float64{10.5, 10.5, 1000.5, 700.5},
			Position:     [8]float64{0, 0, 0, 100, 100, 100, 100, 0},
			Resolution:   &[2]float64{300, 300},
			ColorType:    "Separation",
			Color:        &Color13{CMYK: [4]float64{0, 0, 0, 1}, Name: pdf.String("Black")},
			Tint:         new(0.5),
			Overprint:    true,
			ImageType:    &[2]int{1, 2},
			GrayMap:      []int{0, 21845, 43690, 65535},
			Transparency: new(false),
			Tags: []Tag13{
				{Num: 270, Text: pdf.Text("description")},
				{Num: 271, Text: pdf.Text("make")},
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
				{Num: 270, Text: []pdf.TextString{pdf.Text("a description")}},
				{Num: 333, Text: []pdf.TextString{pdf.Text("first"), pdf.Text("second")}},
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

	w, _ := memfile.NewPDFWriter(t, version, nil)
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
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
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
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
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
		{F: spec("p.tif"), Inks: &Inks20{}},
		{F: spec("p.tif"), Inks: &Inks20{
			Name:       "full_color",
			Monochrome: []Ink20Comp{{Name: pdf.String("Black"), Tint: 1}},
		}},
	}
	for i, v := range bad {
		w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
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
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
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
		w, buf := memfile.NewPDFWriter(f, pdf.V1_7, opt)
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
		// reading accepts versions the writer cannot reproduce
		version := pdf.GetVersion(r)
		if !version.IsSupported() {
			t.Skip("version cannot be written")
		}

		roundTripTest(t, version, d)
	})
}

// newFileSpec returns a closed in-memory writer holding a file specification,
// together with the specification object.
func newFileSpec(t *testing.T) (*pdf.Writer, pdf.Native) {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	fObj, err := rm.Embed(spec("image.tif"))
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return w, fObj
}

func ints(vals ...int) pdf.Array {
	return intsToArray(vals)
}

func nums(vals ...float64) pdf.Array {
	return numbersToArray(vals)
}

// TestV13ReadMalformed checks that malformed optional entries of an OPI 1.3
// dictionary read as absent, and that malformed array elements invalidate
// the whole array instead of being replaced by zero.
func TestV13ReadMalformed(t *testing.T) {
	w, fObj := newFileSpec(t)
	bad := pdf.Name("bad")
	dict := pdf.Dict{
		"Version":      pdf.Number(1.3),
		"F":            fObj,
		"Size":         ints(640, 480),
		"CropRect":     ints(0, 0, 640, 480),
		"CropFixed":    pdf.Array{pdf.Number(0), bad, pdf.Number(640), pdf.Number(480)},
		"Position":     nums(0, 0, 0, 480, 640, 480, 640, 0),
		"Resolution":   pdf.Array{pdf.Number(300), bad},
		"Color":        pdf.Array{pdf.Number(0), pdf.Number(0), bad, pdf.Number(1), pdf.String("Black")},
		"Tint":         bad,
		"ImageType":    pdf.Array{bad, pdf.Integer(8)},
		"GrayMap":      pdf.Array{pdf.Integer(0), bad},
		"Transparency": bad,
		"Tags": pdf.Array{
			bad, pdf.String("dropped"),
			pdf.Integer(270), pdf.String("kept"),
			pdf.Integer(271), bad,
		},
	}
	v, err := extractV13(pdf.CursorAt(pdf.NewExtractor(w), nil), dict, true)
	if err != nil {
		t.Fatal(err)
	}
	want := &V13{
		F:         v.F,
		Size:      [2]int{640, 480},
		CropRect:  [4]int{0, 0, 640, 480},
		Position:  [8]float64{0, 0, 0, 480, 640, 480, 640, 0},
		Tags:      []Tag13{{Num: 270, Text: pdf.Text("kept")}},
		SingleUse: true,
	}
	if diff := cmp.Diff(want, v); diff != "" {
		t.Errorf("malformed entries not dropped (-want +got):\n%s", diff)
	}
}

// TestV13ReadRequired checks the handling of the required Size, CropRect and
// Position entries: a missing or invalid CropRect is recovered from CropFixed,
// a Position with a broken lower-right corner is completed, and otherwise a
// missing or invalid required entry fails the read.
func TestV13ReadRequired(t *testing.T) {
	w, fObj := newFileSpec(t)
	c := pdf.CursorAt(pdf.NewExtractor(w), nil)
	base := func() pdf.Dict {
		return pdf.Dict{
			"Version":  pdf.Number(1.3),
			"F":        fObj,
			"Size":     ints(640, 480),
			"CropRect": ints(0, 0, 640, 480),
			"Position": nums(0, 0, 0, 480, 640, 480, 640, 0),
		}
	}

	dict := base()
	delete(dict, "CropRect")
	dict["CropFixed"] = nums(10.4, 10.6, 600.5, 470)
	v, err := extractV13(c, dict, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := [4]int{10, 10, 601, 470}; v.CropRect != want {
		t.Errorf("recovered CropRect: got %v, want %v", v.CropRect, want)
	}

	dict = base()
	dict["CropRect"] = ints(0, 0, 700, 480)
	dict["CropFixed"] = nums(0, 0, 639.8, 480)
	v, err = extractV13(c, dict, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := [4]int{0, 0, 640, 480}; v.CropRect != want {
		t.Errorf("invalid CropRect: got %v, want %v", v.CropRect, want)
	}

	// OPI 1.3 does not fix the vertical direction, so top > bottom is valid
	dict = base()
	dict["CropRect"] = ints(0, 480, 640, 0)
	dict["CropFixed"] = nums(0.5, 479.5, 639.5, 0.5)
	v, err = extractV13(c, dict, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := [4]int{0, 480, 640, 0}; v.CropRect != want {
		t.Errorf("flipped CropRect: got %v, want %v", v.CropRect, want)
	}
	if v.CropFixed == nil {
		t.Error("flipped CropFixed dropped")
	}

	// outward rounding keeps a narrow crop rectangle non-empty
	dict = base()
	delete(dict, "CropRect")
	dict["CropFixed"] = nums(10.2, 479.5, 10.4, 0.5)
	v, err = extractV13(c, dict, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := [4]int{10, 480, 11, 0}; v.CropRect != want {
		t.Errorf("narrow CropRect from CropFixed: got %v, want %v", v.CropRect, want)
	}

	dict = base()
	dict["Position"] = nums(0, 0, 0, 480, 640, 480, math.Inf(1), 0)
	v, err = extractV13(c, dict, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := [8]float64{0, 0, 0, 480, 640, 480, 640, 0}; v.Position != want {
		t.Errorf("infinite Position corner: got %v, want %v", v.Position, want)
	}

	broken := []struct {
		key pdf.Name
		val pdf.Object
	}{
		{"Size", nil},
		{"Size", pdf.Array{pdf.Integer(640), pdf.Name("bad")}},
		{"Size", ints(640)},
		{"CropRect", nil},
		{"CropRect", pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(640), pdf.Name("bad")}},
		{"CropRect", ints(0, 0, 641, 480)},
		{"CropRect", ints(10, 0, 10, 480)},
		{"CropRect", ints(-1, 0, 640, 480)},
		{"CropRect", ints(0, 100, 640, 100)},
		{"CropRect", ints(0, 481, 640, 0)},
		{"CropFixed", nums(0, 0, 1e300, 480)},
		{"Position", nil},
		{"Position", nums(0, 0, 0, 480, 640, 480, 640)},
		{"Position", nums(0, 0, 0, math.Inf(1), 640, 480, 640, 0)},
		{"Position", nums(0, 0, 0, 480, math.NaN(), 480, 640, 0)},
		{"Position", nums(-1e308, 0, 1e308, 480, 1e308, 480, 640, 0)},
	}
	for _, b := range broken {
		dict := base()
		if b.key == "CropFixed" {
			// CropFixed can only stand in for a missing CropRect
			delete(dict, "CropRect")
		}
		if b.val == nil {
			delete(dict, b.key)
		} else {
			dict[b.key] = b.val
		}
		if _, err := extractV13(c, dict, true); err == nil {
			t.Errorf("%s=%s: expected error", b.key, pdf.AsString(b.val))
		}
	}
}

// TestV20ReadMalformed checks that malformed entries of an OPI 2.0
// dictionary read as absent.
func TestV20ReadMalformed(t *testing.T) {
	w, fObj := newFileSpec(t)
	c := pdf.CursorAt(pdf.NewExtractor(w), nil)
	bad := pdf.Name("bad")
	dict := pdf.Dict{
		"Version":                 pdf.Number(2.0),
		"F":                       fObj,
		"Size":                    pdf.Array{pdf.Number(640), bad},
		"CropRect":                nums(0, 0, 640, 480),
		"IncludedImageDimensions": pdf.Array{bad, pdf.Integer(480)},
		"Tags": pdf.Array{
			pdf.Integer(270), pdf.Array{pdf.String("a"), bad},
			pdf.Integer(271), pdf.Array{pdf.String("b"), pdf.String("c")},
			bad, pdf.String("dropped"),
		},
	}
	v, err := extractV20(c, dict, true)
	if err != nil {
		t.Fatal(err)
	}
	want := &V20{
		F:         v.F,
		Tags:      []Tag20{{Num: 271, Text: []pdf.TextString{pdf.Text("b"), pdf.Text("c")}}},
		SingleUse: true,
	}
	if diff := cmp.Diff(want, v); diff != "" {
		t.Errorf("malformed entries not dropped (-want +got):\n%s", diff)
	}
}

// TestReadInksMalformed checks that an Inks entry with an unknown name, a
// wrong leading name, an odd number of pair elements, or a malformed pair
// reads as absent.
func TestReadInksMalformed(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	c := pdf.CursorAt(pdf.NewExtractor(w), nil)
	cases := []pdf.Object{
		pdf.Name("rainbow"),
		pdf.Array{pdf.Name("duotone"), pdf.String("Black"), pdf.Number(1)},
		pdf.Array{pdf.Name("monochrome"), pdf.String("Black")},
		pdf.Array{pdf.Name("monochrome"), pdf.String("Black"), pdf.Number(1), pdf.String("Cyan"), pdf.Name("bad")},
		pdf.Array{pdf.Name("monochrome"), pdf.Integer(1), pdf.Number(1)},
	}
	for _, obj := range cases {
		inks, err := readInks(c, obj)
		if err != nil {
			t.Fatal(err)
		}
		if inks != nil {
			t.Errorf("readInks(%v) = %+v, want nil", obj, inks)
		}
	}
}

// TestV20WriteInksName checks that Embed rejects an unknown Inks name.
func TestV20WriteInksName(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	v := &V20{F: spec("p.tif"), Inks: &Inks20{Name: "rainbow"}}
	if _, err := rm.Embed(v); err == nil {
		t.Error("expected error, got nil")
	}
}

// failingGetter wraps a [pdf.Getter] and fails to read the object bad with
// an I/O error, which is not a sign of a malformed file.
type failingGetter struct {
	pdf.Getter
	bad pdf.Reference
}

func (g failingGetter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	if ref == g.bad {
		return nil, errors.New("I/O error")
	}
	return g.Getter.Get(ref, canObjStm)
}

// TestReadArrayIOError checks that the array helpers report read errors
// instead of treating them like malformed elements.
func TestReadArrayIOError(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Integer(1)); err != nil {
		t.Fatal(err)
	}
	c := pdf.CursorAt(pdf.NewExtractor(failingGetter{Getter: w, bad: ref}), nil)
	arr := pdf.Array{pdf.Integer(0), ref}
	if _, err := readNumbers(c, arr); err == nil {
		t.Error("readNumbers: expected error")
	}
	if _, err := readInts(c, arr); err == nil {
		t.Error("readInts: expected error")
	}
}

// TestV13ReadRange checks that values outside their valid range are dropped
// on read, and that a Position which is not a parallelogram is repaired.
func TestV13ReadRange(t *testing.T) {
	w, fObj := newFileSpec(t)
	dict := pdf.Dict{
		"Version":   pdf.Number(1.3),
		"F":         fObj,
		"Size":      ints(640, 480),
		"CropRect":  ints(0, 0, 640, 480),
		"CropFixed": nums(0, 0, 700, 480),
		"Position":  nums(0, 0, 0, 480, 640, 480, 600, 10),
		"ColorType": pdf.Name("Rainbow"),
		"Color":     pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(0), pdf.Number(1.5), pdf.String("Black")},
		"Tint":      pdf.Number(-0.5),
		"ImageType": ints(1, 2),
		"GrayMap":   ints(0, 1, 2, 3, 4, 5, 6, 7),
	}
	c := pdf.CursorAt(pdf.NewExtractor(w), nil)
	v, err := extractV13(c, dict, true)
	if err != nil {
		t.Fatal(err)
	}
	want := &V13{
		F:         v.F,
		Size:      [2]int{640, 480},
		CropRect:  [4]int{0, 0, 640, 480},
		Position:  [8]float64{0, 0, 0, 480, 640, 480, 640, 0},
		ImageType: &[2]int{1, 2},
		SingleUse: true,
	}
	if diff := cmp.Diff(want, v); diff != "" {
		t.Errorf("out-of-range entries not fixed (-want +got):\n%s", diff)
	}

	for _, gm := range []pdf.Array{ints(0, 70000), ints(-1, 0), ints(0, 1, 2), ints(make([]int, 1<<17)...)} {
		dict := pdf.Dict{
			"F":        fObj,
			"Size":     ints(640, 480),
			"CropRect": ints(0, 0, 640, 480),
			"Position": nums(0, 0, 0, 480, 640, 480, 640, 0),
			"GrayMap":  gm,
		}
		v, err := extractV13(c, dict, true)
		if err != nil {
			t.Fatal(err)
		}
		if v.GrayMap != nil {
			t.Errorf("dropping GrayMap %v failed", gm)
		}
	}
}

// TestV13WriteValidation checks that Embed rejects out-of-range values.
func TestV13WriteValidation(t *testing.T) {
	valid := func() *V13 {
		return &V13{
			F:        spec("image.tif"),
			Size:     [2]int{640, 480},
			CropRect: [4]int{0, 0, 640, 480},
			Position: [8]float64{0, 0, 0, 480, 640, 480, 640, 0},
		}
	}
	bad := []func(*V13){
		func(v *V13) { v.Size = [2]int{0, 480} },
		func(v *V13) { v.CropRect = [4]int{0, 0, 641, 480} },
		func(v *V13) { v.CropRect = [4]int{0, 480, 640, 480} },
		func(v *V13) { v.CropFixed = &[4]float64{0, 0, 640.5, 480} },
		func(v *V13) { v.Position[6] = 600 },
		func(v *V13) { v.Position[0] = math.Inf(1) },
		func(v *V13) { v.Position = [8]float64{math.NaN(), 0, 0, 480, 640, 480, 640, 0} },
		func(v *V13) { v.ColorType = "Rainbow" },
		func(v *V13) { v.Color = &Color13{CMYK: [4]float64{0, 0, 0, 2}, Name: pdf.String("Black")} },
		func(v *V13) { v.Tint = new(1.5) },
		func(v *V13) { v.GrayMap = []int{0, 1, 2} },
		func(v *V13) { v.GrayMap = []int{0, 70000} },
		func(v *V13) { v.GrayMap = make([]int, 1<<17) },
		func(v *V13) { v.ImageType = &[2]int{1, 8}; v.GrayMap = []int{0, 1, 2, 3} },
	}
	for i, modify := range bad {
		v := valid()
		modify(v)
		w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
		rm := pdf.NewResourceManager(w)
		if _, err := rm.Embed(v); err == nil {
			t.Errorf("case %d: expected error, got nil", i)
		}
	}
}

// TestV20Range checks the CropRect and ink tint range rules on read and
// write.
func TestV20Range(t *testing.T) {
	w, fObj := newFileSpec(t)
	c := pdf.CursorAt(pdf.NewExtractor(w), nil)
	dict := pdf.Dict{
		"F":        fObj,
		"Size":     nums(640, 480),
		"CropRect": nums(0, 0, 700, 480),
		"Inks":     pdf.Array{pdf.Name("monochrome"), pdf.String("Black"), pdf.Number(1.5)},
	}
	v, err := extractV20(c, dict, true)
	if err != nil {
		t.Fatal(err)
	}
	if v.Size != nil || v.CropRect != nil || v.Inks != nil {
		t.Errorf("out-of-range entries not dropped: %+v", v)
	}

	bad := []*V20{
		{F: spec("p.tif"), Size: &[2]float64{640, 480}, CropRect: &[4]float64{0, 0, 700, 480}},
		{F: spec("p.tif"), Size: &[2]float64{640, 480}, CropRect: &[4]float64{100, 0, 100, 480}},
		{F: spec("p.tif"), Inks: &Inks20{Monochrome: []Ink20Comp{{Name: pdf.String("Black"), Tint: -1}}}},
	}
	for i, v := range bad {
		w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
		rm := pdf.NewResourceManager(w)
		if _, err := rm.Embed(v); err == nil {
			t.Errorf("case %d: expected error, got nil", i)
		}
	}
}
