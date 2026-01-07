// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package separation

import (
	"bytes"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var testCases = []struct {
	name string
	dict *Dict
}{
	{
		name: "cyan",
		dict: &Dict{
			DeviceColorant: "Cyan",
		},
	},
	{
		name: "magenta",
		dict: &Dict{
			DeviceColorant: "Magenta",
		},
	},
	{
		name: "pantone",
		dict: &Dict{
			DeviceColorant: "PANTONE 35 CV",
		},
	},
	{
		name: "with_colorspace",
		dict: &Dict{
			DeviceColorant: "PANTONE 123",
			ColorSpace: must(color.Separation("PANTONE 123", color.SpaceDeviceCMYK,
				&function.Type2{
					XMin: 0,
					XMax: 1,
					C0:   []float64{0, 0, 0, 0},
					C1:   []float64{0, 1, 0, 0},
					N:    1,
				})),
		},
	},
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripTest(t, pdf.V1_4, tc.dict)
		})
	}
}

func roundTripTest(t *testing.T, version pdf.Version, d1 *Dict) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)

	// allocate dummy page references if not already set
	if len(d1.Pages) == 0 {
		d1.Pages = []pdf.Reference{w.Alloc(), w.Alloc()}
	}

	rm := pdf.NewResourceManager(w)

	dict, err := d1.Encode(rm)
	var versionError *pdf.VersionError
	if errors.As(err, &versionError) {
		t.Skip("version not supported")
	} else if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("rm.Close failed: %v", err)
	}

	// write dummy page dicts
	for _, ref := range d1.Pages {
		if err := w.Put(ref, pdf.Dict{"Type": pdf.Name("Page")}); err != nil {
			t.Fatalf("Put page failed: %v", err)
		}
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("w.Close failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	d2, err := Decode(x, dict)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	opts := []cmp.Option{
		cmpopts.EquateEmpty(),
		cmp.Comparer(func(a, b color.Space) bool {
			return color.SpacesEqual(a, b)
		}),
	}
	if diff := cmp.Diff(d1, d2, opts...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(pdf.V1_4, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		d := &Dict{
			DeviceColorant: tc.dict.DeviceColorant,
			ColorSpace:     tc.dict.ColorSpace,
			Pages:          []pdf.Reference{w.Alloc()},
		}

		rm := pdf.NewResourceManager(w)

		embedded, err := d.Encode(rm)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		for _, ref := range d.Pages {
			if err := w.Put(ref, pdf.Dict{"Type": pdf.Name("Page")}); err != nil {
				continue
			}
		}

		w.GetMeta().Trailer["Quir:E"] = embedded

		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing object")
		}

		x := pdf.NewExtractor(r)
		d, err := Decode(x, obj)
		if err != nil {
			t.Skip("malformed separation dictionary")
		}

		roundTripTest(t, pdf.GetVersion(r), d)
	})
}

func TestColorSpaceValidation(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_4, nil)

	pageRef := w.Alloc()

	// create a Separation color space with different colorant name
	cs, err := color.Separation("Magenta", color.SpaceDeviceCMYK,
		&function.Type2{
			XMin: 0,
			XMax: 1,
			C0:   []float64{0, 0, 0, 0},
			C1:   []float64{0, 1, 0, 0},
			N:    1,
		})
	if err != nil {
		t.Fatalf("create color space: %v", err)
	}

	d := &Dict{
		Pages:          []pdf.Reference{pageRef},
		DeviceColorant: "Cyan", // mismatched!
		ColorSpace:     cs,
	}

	rm := pdf.NewResourceManager(w)

	_, err = d.Encode(rm)
	if err == nil {
		t.Error("expected error for mismatched colorant, got nil")
	}
}
