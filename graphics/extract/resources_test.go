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

package extract

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/shading"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/property"
)

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		resource *content.Resources
	}{
		{
			name:     "empty",
			resource: &content.Resources{SingleUse: true},
		},
		{
			name: "ExtGState only",
			resource: &content.Resources{
				ExtGState: map[pdf.Name]*extgstate.ExtGState{
					"GS1": {
						Set:       graphics.StateLineWidth,
						LineWidth: 2.0,
					},
				},
				SingleUse: true,
			},
		},
		{
			name: "ColorSpace only",
			resource: &content.Resources{
				ColorSpace: map[pdf.Name]color.Space{
					"CS1": color.SpaceDeviceGray,
				},
				SingleUse: true,
			},
		},
		{
			name: "ProcSet only",
			resource: &content.Resources{
				ProcSet: content.ProcSet{
					PDF:    true,
					Text:   true,
					ImageB: true,
				},
				SingleUse: true,
			},
		},
		{
			name: "ProcSet all flags",
			resource: &content.Resources{
				ProcSet: content.ProcSet{
					PDF:    true,
					Text:   true,
					ImageB: true,
					ImageC: true,
					ImageI: true,
				},
				SingleUse: true,
			},
		},
		{
			name: "multiple types combined",
			resource: &content.Resources{
				ExtGState: map[pdf.Name]*extgstate.ExtGState{
					"GS1": {
						Set:       graphics.StateLineWidth,
						LineWidth: 1.5,
					},
				},
				ColorSpace: map[pdf.Name]color.Space{
					"CS1": color.SpaceDeviceRGB,
					"CS2": color.SpaceDeviceGray,
				},
				ProcSet: content.ProcSet{
					PDF:  true,
					Text: true,
				},
				SingleUse: true,
			},
		},
		{
			name: "SingleUse true (direct dictionary)",
			resource: &content.Resources{
				ProcSet: content.ProcSet{
					PDF: true,
				},
				SingleUse: true,
			},
		},
		{
			name: "SingleUse false (indirect reference)",
			resource: &content.Resources{
				ProcSet: content.ProcSet{
					PDF: true,
				},
				SingleUse: false,
			},
		},
		{
			name: "Properties only",
			resource: &content.Resources{
				Properties: map[pdf.Name]property.List{
					"P1": &property.ActualText{
						Text:      "Test",
						SingleUse: true,
					},
				},
				SingleUse: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create in-memory PDF writer
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)

			// embed the resource
			obj, err := rm.Embed(tt.resource)
			if err != nil {
				t.Fatal(err)
			}

			err = rm.Close()
			if err != nil {
				t.Fatal(err)
			}

			err = w.Close()
			if err != nil {
				t.Fatal(err)
			}

			// extract it back
			x := pdf.NewExtractor(w)
			extracted, err := Resources(x, obj)
			if err != nil {
				t.Fatal(err)
			}

			// manual comparison for Properties field due to interface type
			if len(tt.resource.Properties) != len(extracted.Properties) {
				t.Errorf("Properties count mismatch: want %d, got %d", len(tt.resource.Properties), len(extracted.Properties))
			}
			for name, origProp := range tt.resource.Properties {
				extractedProp, ok := extracted.Properties[name]
				if !ok {
					t.Errorf("missing property %q", name)
					continue
				}
				// compare keys
				origKeys := origProp.Keys()
				extractedKeys := extractedProp.Keys()
				if len(origKeys) != len(extractedKeys) {
					t.Errorf("property %q: key count mismatch: want %d, got %d", name, len(origKeys), len(extractedKeys))
				}
			}

			// compare with cmp.Diff (excluding Properties which we compared manually)
			opts := []cmp.Option{
				cmpopts.EquateEmpty(),
				cmpopts.IgnoreFields(content.Resources{}, "Properties"),
			}
			if diff := cmp.Diff(tt.resource, extracted, opts...); diff != "" {
				t.Errorf("round trip failed (-got +want):\n%s", diff)
			}
		})
	}
}

func TestVersionValidation(t *testing.T) {
	tests := []struct {
		name     string
		version  pdf.Version
		resource *content.Resources
		wantErr  bool
	}{
		{
			name:    "Shading with PDF 1.2 fails",
			version: pdf.V1_2,
			resource: &content.Resources{
				Shading: map[pdf.Name]graphics.Shading{
					"Sh1": &shading.Type1{
						ColorSpace: color.SpaceDeviceGray,
						F: &function.Type4{
							Domain:  []float64{0, 1, 0, 1},
							Range:   []float64{0, 1},
							Program: "add 2 div",
						},
						Domain: []float64{0, 1, 0, 1},
					},
				},
				SingleUse: true,
			},
			wantErr: true,
		},
		{
			name:    "Shading with PDF 1.3 succeeds",
			version: pdf.V1_3,
			resource: &content.Resources{
				Shading: map[pdf.Name]graphics.Shading{
					"Sh1": &shading.Type1{
						ColorSpace: color.SpaceDeviceGray,
						F: &function.Type4{
							Domain:  []float64{0, 1, 0, 1},
							Range:   []float64{0, 1},
							Program: "add 2 div",
						},
						Domain: []float64{0, 1, 0, 1},
					},
				},
				SingleUse: true,
			},
			wantErr: false,
		},
		{
			name:    "Shading with PDF 2.0 succeeds",
			version: pdf.V2_0,
			resource: &content.Resources{
				Shading: map[pdf.Name]graphics.Shading{
					"Sh1": &shading.Type1{
						ColorSpace: color.SpaceDeviceGray,
						F: &function.Type4{
							Domain:  []float64{0, 1, 0, 1},
							Range:   []float64{0, 1},
							Program: "add 2 div",
						},
						Domain: []float64{0, 1, 0, 1},
					},
				},
				SingleUse: true,
			},
			wantErr: false,
		},
		{
			name:    "Properties with PDF 1.1 fails",
			version: pdf.V1_1,
			resource: &content.Resources{
				Properties: map[pdf.Name]property.List{
					"P1": &property.ActualText{
						Text:      "Test",
						SingleUse: true,
					},
				},
				SingleUse: true,
			},
			wantErr: true,
		},
		{
			name:    "Properties with PDF 1.2 succeeds",
			version: pdf.V1_2,
			resource: &content.Resources{
				Properties: map[pdf.Name]property.List{
					"P1": &property.ActualText{
						Text:      "Test",
						SingleUse: true,
					},
				},
				SingleUse: true,
			},
			wantErr: false,
		},
		{
			name:    "Properties with PDF 2.0 succeeds",
			version: pdf.V2_0,
			resource: &content.Resources{
				Properties: map[pdf.Name]property.List{
					"P1": &property.ActualText{
						Text:      "Test",
						SingleUse: true,
					},
				},
				SingleUse: true,
			},
			wantErr: false,
		},
		{
			name:    "ProcSet with PDF 2.0 fails",
			version: pdf.V2_0,
			resource: &content.Resources{
				ProcSet: content.ProcSet{
					PDF: true,
				},
				SingleUse: true,
			},
			wantErr: true,
		},
		{
			name:    "ProcSet with PDF 1.7 succeeds",
			version: pdf.V1_7,
			resource: &content.Resources{
				ProcSet: content.ProcSet{
					PDF: true,
				},
				SingleUse: true,
			},
			wantErr: false,
		},
		{
			name:    "ProcSet with PDF 1.0 succeeds",
			version: pdf.V1_0,
			resource: &content.Resources{
				ProcSet: content.ProcSet{
					PDF:    true,
					Text:   true,
					ImageB: true,
				},
				SingleUse: true,
			},
			wantErr: false,
		},
		{
			name:    "empty ProcSet with PDF 2.0 succeeds",
			version: pdf.V2_0,
			resource: &content.Resources{
				ProcSet:   content.ProcSet{},
				SingleUse: true,
			},
			wantErr: false,
		},
		{
			name:    "multiple features with correct versions succeed",
			version: pdf.V1_7,
			resource: &content.Resources{
				Shading: map[pdf.Name]graphics.Shading{
					"Sh1": &shading.Type1{
						ColorSpace: color.SpaceDeviceGray,
						F: &function.Type4{
							Domain:  []float64{0, 1, 0, 1},
							Range:   []float64{0, 1},
							Program: "add 2 div",
						},
						Domain: []float64{0, 1, 0, 1},
					},
				},
				Properties: map[pdf.Name]property.List{
					"P1": &property.ActualText{
						Text:      "Test",
						SingleUse: true,
					},
				},
				ProcSet: content.ProcSet{
					PDF: true,
				},
				SingleUse: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(tt.version, nil)
			rm := pdf.NewResourceManager(w)

			_, err := rm.Embed(tt.resource)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestProcSetConversion(t *testing.T) {
	tests := []struct {
		name     string
		procSet  content.ProcSet
		expected pdf.Array
	}{
		{
			name:     "all false",
			procSet:  content.ProcSet{},
			expected: nil,
		},
		{
			name: "PDF only",
			procSet: content.ProcSet{
				PDF: true,
			},
			expected: pdf.Array{pdf.Name("PDF")},
		},
		{
			name: "Text only",
			procSet: content.ProcSet{
				Text: true,
			},
			expected: pdf.Array{pdf.Name("Text")},
		},
		{
			name: "ImageB only",
			procSet: content.ProcSet{
				ImageB: true,
			},
			expected: pdf.Array{pdf.Name("ImageB")},
		},
		{
			name: "ImageC only",
			procSet: content.ProcSet{
				ImageC: true,
			},
			expected: pdf.Array{pdf.Name("ImageC")},
		},
		{
			name: "ImageI only",
			procSet: content.ProcSet{
				ImageI: true,
			},
			expected: pdf.Array{pdf.Name("ImageI")},
		},
		{
			name: "PDF and Text",
			procSet: content.ProcSet{
				PDF:  true,
				Text: true,
			},
			expected: pdf.Array{pdf.Name("PDF"), pdf.Name("Text")},
		},
		{
			name: "all images",
			procSet: content.ProcSet{
				ImageB: true,
				ImageC: true,
				ImageI: true,
			},
			expected: pdf.Array{pdf.Name("ImageB"), pdf.Name("ImageC"), pdf.Name("ImageI")},
		},
		{
			name: "all true",
			procSet: content.ProcSet{
				PDF:    true,
				Text:   true,
				ImageB: true,
				ImageC: true,
				ImageI: true,
			},
			expected: pdf.Array{pdf.Name("PDF"), pdf.Name("Text"), pdf.Name("ImageB"), pdf.Name("ImageC"), pdf.Name("ImageI")},
		},
		{
			name: "mixed selection",
			procSet: content.ProcSet{
				PDF:    true,
				ImageC: true,
			},
			expected: pdf.Array{pdf.Name("PDF"), pdf.Name("ImageC")},
		},
		{
			name: "Text and ImageB",
			procSet: content.ProcSet{
				Text:   true,
				ImageB: true,
			},
			expected: pdf.Array{pdf.Name("Text"), pdf.Name("ImageB")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// test embed (booleans → array)
			resource := &content.Resources{
				ProcSet:   tt.procSet,
				SingleUse: true,
			}

			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)

			obj, err := rm.Embed(resource)
			if err != nil {
				t.Fatalf("embed failed: %v", err)
			}

			// verify array in embedded dictionary
			dict, ok := obj.(pdf.Dict)
			if !ok {
				t.Fatalf("expected Dict, got %T", obj)
			}

			procSetArray, hasProcSet := dict["ProcSet"]
			if tt.expected == nil {
				if hasProcSet {
					t.Errorf("expected no ProcSet entry, got %v", procSetArray)
				}
			} else {
				if !hasProcSet {
					t.Error("expected ProcSet entry, got none")
				} else {
					array, ok := procSetArray.(pdf.Array)
					if !ok {
						t.Fatalf("expected Array, got %T", procSetArray)
					}
					if diff := cmp.Diff(tt.expected, array); diff != "" {
						t.Errorf("ProcSet array mismatch (-want +got):\n%s", diff)
					}
				}
			}

			// test round-trip (array → booleans)
			err = rm.Close()
			if err != nil {
				t.Fatal(err)
			}
			err = w.Close()
			if err != nil {
				t.Fatal(err)
			}

			x := pdf.NewExtractor(w)
			extracted, err := Resources(x, obj)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}

			if diff := cmp.Diff(tt.procSet, extracted.ProcSet); diff != "" {
				t.Errorf("round trip failed (-want +got):\n%s", diff)
			}
		})
	}
}

func TestProcSetUnknownNames(t *testing.T) {
	// test that unknown names in PDF array are ignored (permissive)
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	// create a resource dict with ProcSet containing known and unknown names
	dict := pdf.Dict{
		"ProcSet": pdf.Array{
			pdf.Name("PDF"),
			pdf.Name("UnknownName1"),
			pdf.Name("Text"),
			pdf.Name("UnknownName2"),
			pdf.Name("ImageB"),
			pdf.Integer(123), // non-name entry should be ignored
		},
	}

	ref := w.Alloc()
	err := w.Put(ref, dict)
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// extract and verify only known names are preserved
	x := pdf.NewExtractor(w)
	extracted, err := Resources(x, ref)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	expected := content.ProcSet{
		PDF:    true,
		Text:   true,
		ImageB: true,
		ImageC: false,
		ImageI: false,
	}

	if diff := cmp.Diff(expected, extracted.ProcSet); diff != "" {
		t.Errorf("unknown names not ignored correctly (-want +got):\n%s", diff)
	}
}

func FuzzRoundTrip(f *testing.F) {
	// seed corpus with simple test cases from table tests
	testCases := []struct {
		resource *content.Resources
		version  pdf.Version
	}{
		{
			resource: &content.Resources{SingleUse: true},
			version:  pdf.V1_7,
		},
		{
			resource: &content.Resources{
				ProcSet: content.ProcSet{
					PDF:  true,
					Text: true,
				},
				SingleUse: true,
			},
			version: pdf.V1_7,
		},
		{
			resource: &content.Resources{
				ExtGState: map[pdf.Name]*extgstate.ExtGState{
					"GS1": {
						Set:       graphics.StateLineWidth,
						LineWidth: 1.5,
					},
				},
				SingleUse: true,
			},
			version: pdf.V1_7,
		},
		{
			resource: &content.Resources{
				ColorSpace: map[pdf.Name]color.Space{
					"CS1": color.SpaceDeviceRGB,
				},
				SingleUse: true,
			},
			version: pdf.V1_7,
		},
	}

	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(tc.version, nil)
		rm := pdf.NewResourceManager(w)

		obj, err := rm.Embed(tc.resource)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = obj
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		// try to parse as PDF file
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		objPDF := r.GetMeta().Trailer["Quir:E"]
		if objPDF == nil {
			t.Skip("missing test object")
		}

		// first extraction - permissive
		x1 := pdf.NewExtractor(r)
		resource1, err := Resources(x1, objPDF)
		if err != nil {
			t.Skip("extraction failed (permissive)")
		}

		// embed back - strict
		w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
		rm := pdf.NewResourceManager(w)

		embedded, err := rm.Embed(resource1)
		if err != nil {
			t.Fatalf("embed failed after successful extraction: %v", err)
		}

		err = rm.Close()
		if err != nil {
			t.Fatalf("resource manager close failed: %v", err)
		}

		err = w.Close()
		if err != nil {
			t.Fatalf("writer close failed: %v", err)
		}

		// second extraction
		x2 := pdf.NewExtractor(w)
		resource2, err := Resources(x2, embedded)
		if err != nil {
			t.Fatalf("second extraction failed: %v", err)
		}

		// compare with cmp.Diff (excluding Properties which requires special handling)
		opts := []cmp.Option{
			cmpopts.EquateEmpty(),
			cmpopts.IgnoreFields(content.Resources{}, "Properties"),
		}
		if diff := cmp.Diff(resource1, resource2, opts...); diff != "" {
			t.Errorf("round trip failed (-first +second):\n%s", diff)
		}

		// manual comparison for Properties field
		if len(resource1.Properties) != len(resource2.Properties) {
			t.Errorf("Properties count mismatch: first %d, second %d", len(resource1.Properties), len(resource2.Properties))
		}
	})
}
