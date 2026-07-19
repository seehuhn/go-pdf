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

package decode

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/text/language"
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/trapnet"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/pdf/sound"
)

func makeDefaultAppearance() *form.Form {
	b := builder.New(content.Form, nil, pdf.V2_0)
	b.SetFillColor(color.DeviceGray(0.5))
	b.Rectangle(0, 0, 24, 24)
	b.Fill()
	return &form.Form{
		Content: &content.Operators{Ops: b.Stream},
		Res:     b.Resources,
		BBox:    pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
		Matrix:  matrix.Identity,
	}
}

// makeTrapNetAppearance returns an appearance which is valid for a trap
// network annotation: the normal appearance of such an annotation is the trap
// network itself and has to carry the trap network entries.
func makeTrapNetAppearance() *form.Form {
	f := makeDefaultAppearance()
	f.TrapNet = &trapnet.Attributes{PCM: trapnet.DefaultPCM}
	return f
}

var (
	defaultAppearance     = makeDefaultAppearance()
	defaultAppearanceDict = &appearance.Dict{
		Normal:   defaultAppearance,
		RollOver: defaultAppearance,
		Down:     defaultAppearance,
	}
	trapNetAppearance     = makeTrapNetAppearance()
	trapNetAppearanceDict = &appearance.Dict{
		Normal:   trapNetAppearance,
		RollOver: trapNetAppearance,
		Down:     trapNetAppearance,
	}
)

// appearanceFor returns an appearance dictionary which a can legally use.
func appearanceFor(a annotation.Annotation) *appearance.Dict {
	if _, isTrapNet := a.(*annotation.TrapNet); isTrapNet {
		return trapNetAppearanceDict
	}
	return defaultAppearanceDict
}

func TestRoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for annotationType, cases := range testCases {
			for _, tc := range cases {
				t.Run(fmt.Sprintf("%s-%s-%s", annotationType, tc.name, v), func(t *testing.T) {
					roundTripValue(t, v, tc.annotation)
				})
			}
		}
	}
}

func TestRoundTripDict(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, dict := range testDicts {
			t.Run(fmt.Sprintf("dict-%d", i), func(t *testing.T) {
				// make sure Decode does not crash or hang
				x := pdf.NewExtractor(mock.Getter)
				a, err := Annotation(pdf.CursorAt(x, nil), dict, false)
				if err != nil {
					t.Error(err)
					return
				}

				// if we managed to decode an annotation, do a round-trip test
				roundTripFile(t, v, a)
			})
		}
	}
}

// roundTripValue performs a round-trip test for an annotation constructed in
// Go.  Test cases are written without an appearance dictionary where it is not
// the point of the case, so one is supplied here wherever the annotation needs
// it.  Asking [annotation.AppearanceRequired] rather than checking the version
// keeps this in step with the rule the writer enforces.
//
// Only use this for hand-written values.  Anything read from a file must go
// through [roundTripFile], which does not touch the value it is given: the
// whole point of a round trip is that whatever we can read, we can write back
// unchanged, and completing the value here would hide exactly that failure.
func roundTripValue(t *testing.T, v pdf.Version, a1 annotation.Annotation) {
	c := a1.GetCommon()
	if c.Appearance == nil && annotation.AppearanceRequired(a1.AnnotationType(), c.Rect, v) {
		a1 = shallowCopy(a1)
		a1.GetCommon().Appearance = appearanceFor(a1)
	}
	roundTripFile(t, v, a1)
}

// roundTripFile performs a round-trip test for any annotation type, writing
// a1 exactly as given and comparing the result of reading it back.
func roundTripFile(t *testing.T, v pdf.Version, a1 annotation.Annotation) {
	buf, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(buf)

	// embed the annotation
	dict, err := a1.Encode(rm)
	if pdf.IsWrongVersion(err) {
		t.Skip()
	} else if err != nil {
		t.Fatalf("cannot write the annotation back: %v", err)
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
	a2, err := Annotation(pdf.CursorAt(x, nil), dict, false)
	if err != nil {
		t.Fatal(err)
	}

	// Use cmp options to handle semantic equivalence in round-trip comparison
	opts := []cmp.Option{
		cmp.AllowUnexported(language.Tag{}),
		cmpopts.EquateComparable(language.Tag{}),
		// a widget's Parent (its form field) is the back-edge of the field/widget
		// cycle and is set by the field tree, not on a standalone widget; ignore it
		cmpopts.IgnoreFields(annotation.Widget{}, "Field"),
		// Sound sample data is supplied through closures or stream
		// wrappers; round-trip of the bytes is exercised by the sound
		// package's own tests.
		cmpopts.IgnoreFields(sound.Sound{}, "Data"),
		// Use form.Equal for comparing forms, which handles nil vs empty Args
		// and ignores resource differences (SingleUse, etc.)
		cmp.Comparer(func(a, b *form.Form) bool {
			if a == nil && b == nil {
				return true
			}
			if a == nil || b == nil {
				return false
			}
			result := a.Equal(b)
			if !result {
				t.Logf("form.Equal=false: a.Content=%+v", a.Content)
				t.Logf("form.Equal=false: b.Content=%+v", b.Content)
			}
			return result
		}),
	}

	if diff := cmp.Diff(a1, a2, opts...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func shallowCopy(iface annotation.Annotation) annotation.Annotation {
	origVal := reflect.ValueOf(iface)

	if origVal.Kind() != reflect.Pointer {
		return iface
	}

	elemType := origVal.Elem().Type()
	newPtr := reflect.New(elemType)
	newPtr.Elem().Set(origVal.Elem())

	return newPtr.Interface().(annotation.Annotation)
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
			annot := &annotation.Text{
				Common: annotation.Common{
					Rect:                    pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
					StrokingTransparency:    tt.strokeTransparency,
					NonStrokingTransparency: tt.nonStrokeTransparency,
					Appearance:              defaultAppearanceDict,
				},
			}

			buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(buf)

			embedded, err := annot.Encode(rm)
			if err != nil {
				t.Fatal(err)
			}

			err = rm.Close()
			if err != nil {
				t.Fatal(err)
			}

			dict, err := pdf.NewCursor(rm.Out).Dict(embedded)
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
			common.Appearance = appearanceFor(a)
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
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skipf("invalid PDF: %s", err)
		}
		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing annotation")
		}
		x := pdf.NewExtractor(r)
		annot, err := Annotation(pdf.CursorAt(x, nil), obj, false)
		if err != nil {
			t.Skip("broken annotation")
		}

		// Make sure we can write the annotation, and read it back.
		roundTripFile(t, pdf.GetVersion(r), annot)
	})
}
