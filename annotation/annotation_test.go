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

package annotation

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/text/language"
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

var (
	defaultAppearance = &form.Form{
		Draw: func(page *graphics.Writer) error {
			page.SetFillColor(color.DeviceGray(0.5))
			page.Rectangle(0, 0, 24, 24)
			page.Fill()
			return nil
		},
		BBox:   pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
		Matrix: matrix.Identity,
	}
	defaultAppearanceDict = &appearance.Dict{
		Normal:   defaultAppearance,
		RollOver: defaultAppearance,
		Down:     defaultAppearance,
	}
)

func TestRoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for annotationType, cases := range testCases {
			for _, tc := range cases {
				t.Run(fmt.Sprintf("%s-%s-%s", annotationType, tc.name, v), func(t *testing.T) {
					a := tc.annotation

					if v >= pdf.V2_0 { // add an appearance dictionary, if needed
						a = shallowCopy(a)
						c := a.GetCommon()
						c.Appearance = defaultAppearanceDict
					}

					roundTripTest(t, v, a)
				})
			}
		}
	}
}

func TestRoundTripDict(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, dict := range testDicts {
			t.Run(fmt.Sprintf("dict-%d", i), func(t *testing.T) {
				// make sure Extract does not crash or hang
				x := pdf.NewExtractor(mock.Getter)
				a, err := Decode(x, dict)
				if err != nil {
					t.Error(err)
					return
				}

				// add an appearance dictionary, if needed
				if v >= pdf.V2_0 {
					a = shallowCopy(a)
					c := a.GetCommon()
					c.Appearance = defaultAppearanceDict
				}

				// if we managed to extract an annotation, do a round-trip test
				roundTripTest(t, v, a)
			})
		}
	}
}

// roundTripTest performs a round-trip test for any annotation type
func roundTripTest(t *testing.T, v pdf.Version, a1 Annotation) {
	buf, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(buf)

	// embed the annotation
	dict, err := a1.Encode(rm)
	var versionError *pdf.VersionError
	if errors.As(err, &versionError) {
		t.Skip()
	} else if err != nil {
		t.Fatal(err)
	}
	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = buf.Close()
	if err != nil {
		t.Fatal(err)
	}

	// some special case checks on the encoded dict
	if dict, ok := dict.(pdf.Dict); ok {
		_, hasBorder := dict["Border"]
		_, hasBS := dict["BS"]
		if hasBorder && hasBS {
			t.Errorf("%T annotation has both Border and BS entries", a1)
		}
	}

	// read back
	x := pdf.NewExtractor(buf)
	a2, err := Decode(x, dict)
	if err != nil {
		t.Fatal(err)
	}

	// Use EquateComparable to handle language.Tag comparison
	opts := []cmp.Option{
		cmp.AllowUnexported(language.Tag{}),
		cmpopts.EquateComparable(language.Tag{}),
	}

	if diff := cmp.Diff(a1, a2, opts...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func shallowCopy(iface Annotation) Annotation {
	origVal := reflect.ValueOf(iface)

	if origVal.Kind() != reflect.Pointer {
		return iface
	}

	elemType := origVal.Elem().Type()
	newPtr := reflect.New(elemType)
	newPtr.Elem().Set(origVal.Elem())

	return newPtr.Interface().(Annotation)
}

func TestAnnotationTypes(t *testing.T) {
	for annotationType, cases := range testCases {
		for _, tc := range cases {
			if tc.annotation.AnnotationType() != annotationType {
				t.Errorf("expected annotation type %q, got %q", annotationType, tc.annotation.AnnotationType())
			}
		}
	}
}

func TestOpacity(t *testing.T) {
	tests := []struct {
		name                  string
		strokeTransparency    float64
		nonStrokeTransparency float64
		expectCA              bool
		expectCa              bool
	}{
		{
			name:                  "default transparencies",
			strokeTransparency:    0.0,
			nonStrokeTransparency: 0.0,
			expectCA:              false,
			expectCa:              false,
		},
		{
			name:                  "custom stroke transparency",
			strokeTransparency:    0.5,
			nonStrokeTransparency: 0.5,
			expectCA:              true,
			expectCa:              false,
		},
		{
			name:                  "different transparencies",
			strokeTransparency:    0.2,
			nonStrokeTransparency: 0.4,
			expectCA:              true,
			expectCa:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotation := &Text{
				Common: Common{
					Rect:                    pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
					StrokingTransparency:    tt.strokeTransparency,
					NonStrokingTransparency: tt.nonStrokeTransparency,
					Appearance:              defaultAppearanceDict,
				},
			}

			buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(buf)

			embedded, err := annotation.Encode(rm)
			if err != nil {
				t.Fatal(err)
			}

			err = rm.Close()
			if err != nil {
				t.Fatal(err)
			}

			dict, err := pdf.GetDict(rm.Out, embedded)
			if err != nil {
				t.Fatal(err)
			}

			_, hasCA := dict["CA"]
			_, hasCa := dict["ca"]

			if hasCA != tt.expectCA {
				t.Errorf("CA entry presence: got %v, want %v", hasCA, tt.expectCA)
			}
			if hasCa != tt.expectCa {
				t.Errorf("ca entry presence: got %v, want %v", hasCa, tt.expectCa)
			}
		})
	}
}

func FuzzRoundTrip(f *testing.F) {
	// Seed the fuzzer with valid test cases from all annotation types
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, cases := range testCases {
		for _, tc := range cases {
			w, buf := memfile.NewPDFWriter(pdf.V2_0, opt)
			rm := pdf.NewResourceManager(w)

			err := memfile.AddBlankPage(w)
			if err != nil {
				continue
			}

			a := shallowCopy(tc.annotation)
			common := a.GetCommon()
			common.Appearance = defaultAppearanceDict
			common.AppearanceState = pdf.Name("Normal")

			embedded, err := a.Encode(rm)
			if err != nil {
				continue
			}

			err = rm.Close()
			if err != nil {
				continue
			}
			w.GetMeta().Trailer["Quir:E"] = embedded
			err = w.Close()
			if err != nil {
				continue
			}

			f.Add(buf.Data)
		}
	}
	for _, dict := range testDicts {
		w, out := memfile.NewPDFWriter(pdf.V1_7, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = dict

		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(out.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		// Make sure we don't panic on random input.
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skipf("invalid PDF: %s", err)
		}
		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing annotation")
		}
		x := pdf.NewExtractor(r)
		annotation, err := Decode(x, obj)
		if err != nil {
			t.Skip("broken annotation")
		}

		// Make sure we can write the annotation, and read it back.
		roundTripTest(t, pdf.GetVersion(r), annotation)
	})
}
