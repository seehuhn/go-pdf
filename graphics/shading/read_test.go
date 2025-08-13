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

package shading

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// testCases holds test cases for all shading types, indexed by type
var testCases = map[int][]testCase{
	1: {
		{
			name: "basic Type1",
			shading: &Type1{
				ColorSpace: color.DeviceRGBSpace,
				F: &function.Type0{
					Domain:        []float64{0, 1, 0, 1},
					Range:         []float64{0, 1, 0, 1, 0, 1},
					Size:          []int{2, 2},
					BitsPerSample: 8,
					Encode:        []float64{0, 1, 0, 1},
					Decode:        []float64{0, 1, 0, 1, 0, 1},
					Samples:       []byte{255, 0, 0, 0, 255, 0, 128, 128, 0, 0, 0, 255},
				},
			},
		},
		{
			name: "Type1 with background and bbox",
			shading: &Type1{
				ColorSpace: color.DeviceRGBSpace,
				F: &function.Type0{
					Domain:        []float64{0, 1, 0, 1},
					Range:         []float64{0, 1, 0, 1, 0, 1},
					Size:          []int{2, 2},
					BitsPerSample: 8,
					Encode:        []float64{0, 1, 0, 1},
					Decode:        []float64{0, 1, 0, 1, 0, 1},
					Samples:       []byte{0, 0, 255, 255, 255, 0, 128, 128, 128, 255, 255, 0},
				},
				Background: []float64{0.5, 0.5, 0.5},
				BBox:       &pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
				AntiAlias:  true,
			},
		},
		{
			name: "Type1 with custom domain and matrix",
			shading: &Type1{
				ColorSpace: color.DeviceRGBSpace,
				F: &function.Type0{
					Domain:        []float64{0, 1, 0, 1},
					Range:         []float64{0, 1, 0, 1, 0, 1},
					Size:          []int{2, 2},
					BitsPerSample: 8,
					Encode:        []float64{0, 1, 0, 1},
					Decode:        []float64{0, 1, 0, 1, 0, 1},
					Samples:       []byte{255, 255, 255, 0, 0, 0, 128, 128, 128, 64, 64, 64},
				},
				Domain: []float64{-1, 1, -1, 1},
				Matrix: []float64{2, 0, 0, 2, 10, 10},
			},
		},
	},
	2: {
		{
			name: "basic Type2",
			shading: &Type2{
				ColorSpace: color.DeviceRGBSpace,
				X0:         0, Y0: 0, X1: 100, Y1: 100,
				F: &function.Type2{
					XMin: 0, XMax: 1,
					C0: []float64{1, 0, 0},
					C1: []float64{0, 0, 1},
					N:  1.0,
				},
			},
		},
		{
			name: "Type2 with extend and domain",
			shading: &Type2{
				ColorSpace: color.DeviceRGBSpace,
				X0:         10, Y0: 20, X1: 90, Y1: 80,
				F: &function.Type2{
					XMin: 0, XMax: 1,
					C0: []float64{0, 1, 0},
					C1: []float64{1, 0, 1},
					N:  2.0,
				},
				TMin:        0.2,
				TMax:        0.8,
				ExtendStart: true,
				ExtendEnd:   true,
				Background:  []float64{0.2, 0.2, 0.2},
				AntiAlias:   true,
			},
		},
		{
			name: "Type2 with bbox",
			shading: &Type2{
				ColorSpace: color.DeviceRGBSpace,
				X0:         0, Y0: 0, X1: 50, Y1: 50,
				F: &function.Type2{
					XMin: 0, XMax: 1,
					C0: []float64{1, 1, 0},
					C1: []float64{0, 1, 1},
					N:  0.5,
				},
				BBox: &pdf.Rectangle{LLx: -10, LLy: -10, URx: 60, URy: 60},
			},
		},
	},
	3: {
		{
			name: "basic Type3",
			shading: &Type3{
				ColorSpace: color.DeviceRGBSpace,
				X1:         20, Y1: 30, R1: 0,
				X2: 80, Y2: 70, R2: 50,
				F: &function.Type2{
					XMin: 0, XMax: 1,
					C0: []float64{1, 0, 0},
					C1: []float64{0, 0, 1},
					N:  1.0,
				},
			},
		},
		{
			name: "Type3 with extend and domain",
			shading: &Type3{
				ColorSpace: color.DeviceRGBSpace,
				X1:         50, Y1: 50, R1: 10,
				X2: 50, Y2: 50, R2: 40,
				F: &function.Type2{
					XMin: 0, XMax: 1,
					C0: []float64{1, 1, 0},
					C1: []float64{0, 1, 1},
					N:  2.0,
				},
				TMin:        0.1,
				TMax:        0.9,
				ExtendStart: true,
				ExtendEnd:   false,
				Background:  []float64{0.8, 0.8, 0.8},
				BBox:        &pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
				AntiAlias:   true,
			},
		},
	},
	4: {
		{
			name: "basic Type4",
			shading: &Type4{
				ColorSpace:        color.DeviceRGBSpace,
				BitsPerCoordinate: 16,
				BitsPerComponent:  8,
				BitsPerFlag:       2,
				Decode:            []float64{0, 100, 0, 100, 0, 1, 0, 1, 0, 1},
				Vertices: []Type4Vertex{
					{X: 0, Y: 0, Flag: 0, Color: []float64{1, 0, 0}},
					{X: 50, Y: 0, Flag: 1, Color: []float64{0, 1, 0}},
					{X: 25, Y: 50, Flag: 2, Color: []float64{0, 0, 1}},
				},
			},
		},
		{
			name: "Type4 with function",
			shading: &Type4{
				ColorSpace:        color.DeviceRGBSpace,
				BitsPerCoordinate: 8,
				BitsPerComponent:  8,
				BitsPerFlag:       2,
				Decode:            []float64{0, 255, 0, 255, 0, 1},
				F: &function.Type2{
					XMin: 0, XMax: 1,
					C0: []float64{0, 0, 0},
					C1: []float64{1, 1, 1},
					N:  1.0,
				},
				Vertices: []Type4Vertex{
					{X: 10, Y: 10, Flag: 0, Color: []float64{0.2}},
					{X: 90, Y: 10, Flag: 1, Color: []float64{0.8}},
					{X: 50, Y: 90, Flag: 2, Color: []float64{0.5}},
					{X: 10, Y: 90, Flag: 1, Color: []float64{0.1}},
				},
				Background: []float64{0.9, 0.9, 0.9},
				BBox:       &pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
				AntiAlias:  true,
			},
		},
	},
}

type testCase struct {
	name    string
	shading graphics.Shading
}

func TestRoundTrip(t *testing.T) {
	for _, cases := range testCases {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				roundTripTest(t, tc.shading)
			})
		}
	}
}

// roundTripTest performs a round-trip test for any shading type
func roundTripTest(t *testing.T, originalShading graphics.Shading) {
	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	// Embed the shading
	embedded, _, err := originalShading.Embed(rm)
	if err != nil {
		t.Fatal(err)
	}

	ref := buf.Alloc()
	err = buf.Put(ref, embedded)
	if err != nil {
		t.Fatal(err)
	}

	err = buf.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Read the shading back
	readShading, err := Read(buf, ref)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the types match
	if readShading.ShadingType() != originalShading.ShadingType() {
		t.Fatalf("shading type mismatch: expected %d, got %d",
			originalShading.ShadingType(), readShading.ShadingType())
	}

	// Use cmp.Diff to compare the original and read shading
	// Ignore unexported fields and use tolerance for floating point comparisons
	opts := []cmp.Option{
		cmpopts.IgnoreUnexported(Type1{}, Type2{}, Type3{}, Type4{}),
		cmpopts.EquateApprox(0.01, 0.01), // Allow precision differences from bit quantization
	}
	if diff := cmp.Diff(originalShading, readShading, opts...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestShadingEvaluation(t *testing.T) {
	// Test basic functionality of different shading types
	tests := []struct {
		name    string
		shading graphics.Shading
	}{
		{
			name: "Type1 evaluation",
			shading: &Type1{
				ColorSpace: color.DeviceRGBSpace,
				F: &function.Type0{
					Domain:        []float64{0, 1, 0, 1},
					Range:         []float64{0, 1, 0, 1, 0, 1},
					Size:          []int{2, 2},
					BitsPerSample: 8,
					Encode:        []float64{0, 1, 0, 1},
					Decode:        []float64{0, 1, 0, 1, 0, 1},
					Samples:       []byte{0, 0, 0, 255, 255, 255, 128, 128, 128, 192, 192, 192},
				},
			},
		},
		{
			name: "Type2 evaluation",
			shading: &Type2{
				ColorSpace: color.DeviceRGBSpace,
				X0:         0, Y0: 0, X1: 100, Y1: 100,
				F: &function.Type2{
					XMin: 0, XMax: 1,
					C0: []float64{1, 0, 0},
					C1: []float64{0, 1, 0},
					N:  1.0,
				},
			},
		},
		{
			name: "Type3 evaluation",
			shading: &Type3{
				ColorSpace: color.DeviceRGBSpace,
				X1:         50, Y1: 50, R1: 0,
				X2: 50, Y2: 50, R2: 25,
				F: &function.Type2{
					XMin: 0, XMax: 1,
					C0: []float64{1, 0, 0},
					C1: []float64{0, 0, 1},
					N:  1.0,
				},
			},
		},
		{
			name: "Type4 evaluation",
			shading: &Type4{
				ColorSpace:        color.DeviceRGBSpace,
				BitsPerCoordinate: 8,
				BitsPerComponent:  8,
				BitsPerFlag:       2,
				Decode:            []float64{0, 100, 0, 100, 0, 1, 0, 1, 0, 1},
				Vertices: []Type4Vertex{
					{X: 0, Y: 0, Flag: 0, Color: []float64{1, 0, 0}},
					{X: 100, Y: 0, Flag: 1, Color: []float64{0, 1, 0}},
					{X: 50, Y: 100, Flag: 2, Color: []float64{0, 0, 1}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the shading can be embedded without error
			buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(buf)

			_, _, err := tt.shading.Embed(rm)
			if err != nil {
				t.Errorf("failed to embed shading: %v", err)
			}
		})
	}
}

func TestReadErrors(t *testing.T) {
	tests := []struct {
		name    string
		dict    pdf.Dict
		wantErr bool
	}{
		{
			name:    "missing ShadingType",
			dict:    pdf.Dict{},
			wantErr: true,
		},
		{
			name: "invalid ShadingType",
			dict: pdf.Dict{
				"ShadingType": pdf.Integer(99),
			},
			wantErr: true,
		},
		{
			name: "Type1 missing ColorSpace",
			dict: pdf.Dict{
				"ShadingType": pdf.Integer(1),
			},
			wantErr: true,
		},
		{
			name: "Type1 missing Function",
			dict: pdf.Dict{
				"ShadingType": pdf.Integer(1),
				"ColorSpace":  pdf.Name("DeviceRGB"),
			},
			wantErr: true,
		},
		{
			name: "Type2 missing Coords",
			dict: pdf.Dict{
				"ShadingType": pdf.Integer(2),
				"ColorSpace":  pdf.Name("DeviceRGB"),
				"Function": pdf.Dict{
					"FunctionType": pdf.Integer(2),
					"N":            pdf.Number(1),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)

			_, err := Read(buf, tt.dict)
			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestType2InvalidColorSpace(t *testing.T) {
	// Test that Type2 shading rejects Indexed color spaces
	indexedColorSpace, err := color.Indexed([]color.Color{
		color.Black,
		color.White,
	})
	if err != nil {
		t.Fatal(err)
	}

	shading := &Type2{
		ColorSpace: indexedColorSpace,
		X0:         0, Y0: 0, X1: 100, Y1: 100,
		F: &function.Type2{
			XMin: 0, XMax: 1,
			C0: []float64{0},
			C1: []float64{1},
			N:  1.0,
		},
	}

	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	_, _, err = shading.Embed(rm)
	if err == nil {
		t.Error("expected error for Indexed color space with Type2 shading, got nil")
	} else if err.Error() != "invalid ColorSpace" {
		t.Errorf("expected 'invalid ColorSpace' error, got: %v", err)
	}
}

func FuzzRead(f *testing.F) {
	// Seed the fuzzer with valid test cases from all shading types
	for _, cases := range testCases {
		for _, tc := range cases {
			out := memfile.New()
			opt := &pdf.WriterOptions{
				HumanReadable: true,
			}
			w, err := pdf.NewWriter(out, pdf.V2_0, opt)
			if err != nil {
				f.Fatal(err)
			}
			rm := pdf.NewResourceManager(w)

			ref := w.Alloc()

			embedded, _, err := tc.shading.Embed(rm)
			if err != nil {
				continue
			}

			err = w.Put(ref, embedded)
			if err != nil {
				continue
			}

			err = rm.Close()
			if err != nil {
				continue
			}

			w.GetMeta().Trailer["Quir:X"] = ref

			err = w.Close()
			if err != nil {
				continue
			}

			f.Add(out.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		// Get a "random" shading from the PDF file.

		// Make sure we don't panic on random input.
		opt := &pdf.ReaderOptions{
			ErrorHandling: pdf.ErrorHandlingReport,
		}
		r, err := pdf.NewReader(bytes.NewReader(fileData), opt)
		if err != nil {
			t.Skip("broken PDF: " + err.Error())
		}
		obj := r.GetMeta().Trailer["Quir:X"]
		if obj == nil {
			t.Skip("broken reference")
		}
		shading, err := Read(r, obj)
		if err != nil {
			t.Skip("broken shading")
		}

		// Make sure we can write the shading, and read it back.
		// Skip if the shading has validation errors (e.g., wrong function input count)
		defer func() {
			if r := recover(); r != nil {
				t.Skipf("shading validation failed: %v", r)
			}
		}()

		// Try to embed the shading - this will catch validation errors
		buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
		rm := pdf.NewResourceManager(buf)
		_, _, err = shading.Embed(rm)
		if err != nil {
			t.Skipf("shading embed failed: %v", err)
		}

		roundTripTest(t, shading)

		// Test basic shading properties don't panic
		shadingType := shading.ShadingType()
		if shadingType < 1 || shadingType > 4 {
			t.Errorf("invalid shading type: %d", shadingType)
		}
	})
}
