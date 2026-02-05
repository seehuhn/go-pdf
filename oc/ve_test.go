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

package oc

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var visibilityExpressionTestCases = []struct {
	name    string
	version pdf.Version
	ve      VisibilityExpression
}{
	{
		name:    "simple group",
		version: pdf.V1_7,
		ve:      &VisibilityExpressionGroup{Group: &Group{Name: "Layer1"}},
	},
	{
		name:    "And with two groups",
		version: pdf.V1_7,
		ve: &VisibilityExpressionAnd{
			Args: []VisibilityExpression{
				&VisibilityExpressionGroup{Group: &Group{Name: "Layer1"}},
				&VisibilityExpressionGroup{Group: &Group{Name: "Layer2"}},
			},
		},
	},
	{
		name:    "And with three groups",
		version: pdf.V2_0,
		ve: &VisibilityExpressionAnd{
			Args: []VisibilityExpression{
				&VisibilityExpressionGroup{Group: &Group{Name: "A"}},
				&VisibilityExpressionGroup{Group: &Group{Name: "B"}},
				&VisibilityExpressionGroup{Group: &Group{Name: "C"}},
			},
		},
	},
	{
		name:    "Or with two groups",
		version: pdf.V1_7,
		ve: &VisibilityExpressionOr{
			Args: []VisibilityExpression{
				&VisibilityExpressionGroup{Group: &Group{Name: "Layer1"}},
				&VisibilityExpressionGroup{Group: &Group{Name: "Layer2"}},
			},
		},
	},
	{
		name:    "Not with group",
		version: pdf.V1_7,
		ve: &VisibilityExpressionNot{
			Arg: &VisibilityExpressionGroup{Group: &Group{Name: "HiddenLayer"}},
		},
	},
	{
		name:    "nested And in Or",
		version: pdf.V2_0,
		ve: &VisibilityExpressionOr{
			Args: []VisibilityExpression{
				&VisibilityExpressionGroup{Group: &Group{Name: "Layer1"}},
				&VisibilityExpressionAnd{
					Args: []VisibilityExpression{
						&VisibilityExpressionGroup{Group: &Group{Name: "Layer2"}},
						&VisibilityExpressionGroup{Group: &Group{Name: "Layer3"}},
					},
				},
			},
		},
	},
	{
		name:    "nested Not in And",
		version: pdf.V1_7,
		ve: &VisibilityExpressionAnd{
			Args: []VisibilityExpression{
				&VisibilityExpressionGroup{Group: &Group{Name: "Visible"}},
				&VisibilityExpressionNot{
					Arg: &VisibilityExpressionGroup{Group: &Group{Name: "Hidden"}},
				},
			},
		},
	},
	{
		name:    "complex nested expression",
		version: pdf.V2_0,
		ve: &VisibilityExpressionOr{
			Args: []VisibilityExpression{
				&VisibilityExpressionGroup{Group: &Group{Name: "OCG1"}},
				&VisibilityExpressionNot{
					Arg: &VisibilityExpressionGroup{Group: &Group{Name: "OCG2"}},
				},
				&VisibilityExpressionAnd{
					Args: []VisibilityExpression{
						&VisibilityExpressionGroup{Group: &Group{Name: "OCG3"}},
						&VisibilityExpressionGroup{Group: &Group{Name: "OCG4"}},
						&VisibilityExpressionGroup{Group: &Group{Name: "OCG5"}},
					},
				},
			},
		},
	},
	{
		name:    "deeply nested",
		version: pdf.V2_0,
		ve: &VisibilityExpressionAnd{
			Args: []VisibilityExpression{
				&VisibilityExpressionOr{
					Args: []VisibilityExpression{
						&VisibilityExpressionNot{
							Arg: &VisibilityExpressionGroup{Group: &Group{Name: "Deep"}},
						},
						&VisibilityExpressionGroup{Group: &Group{Name: "Also"}},
					},
				},
				&VisibilityExpressionGroup{Group: &Group{Name: "Top"}},
			},
		},
	},
}

func TestVisibilityExpressionRoundTrip(t *testing.T) {
	for _, tc := range visibilityExpressionTestCases {
		t.Run(tc.name, func(t *testing.T) {
			testVisibilityExpressionRoundTrip(t, tc.version, tc.ve)
		})
	}
}

func testVisibilityExpressionRoundTrip(t *testing.T, version pdf.Version, original VisibilityExpression) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	obj, err := rm.Embed(original)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("embed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("close resource manager: %v", err)
	}

	extractor := pdf.NewExtractor(w)
	extracted, err := ExtractVisibilityExpression(extractor, obj)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	normalizeVisibilityExpression(original)
	normalizeVisibilityExpression(extracted)

	if diff := cmp.Diff(original, extracted); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestVisibilityExpressionVersionCheck(t *testing.T) {
	// visibility expressions require PDF 1.6+
	w, _ := memfile.NewPDFWriter(pdf.V1_5, nil)
	rm := pdf.NewResourceManager(w)

	ve := &VisibilityExpressionAnd{
		Args: []VisibilityExpression{
			&VisibilityExpressionGroup{Group: &Group{Name: "Layer1"}},
			&VisibilityExpressionGroup{Group: &Group{Name: "Layer2"}},
		},
	}

	_, err := rm.Embed(ve)
	if err == nil {
		t.Error("expected error for PDF 1.5, got nil")
	}

	rm.Close()
}

func TestVisibilityExpressionValidation(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	tests := []struct {
		name string
		ve   VisibilityExpression
	}{
		{
			name: "And with empty args",
			ve:   &VisibilityExpressionAnd{Args: []VisibilityExpression{}},
		},
		{
			name: "Or with empty args",
			ve:   &VisibilityExpressionOr{Args: []VisibilityExpression{}},
		},
		{
			name: "Not with nil arg",
			ve:   &VisibilityExpressionNot{Arg: nil},
		},
		{
			name: "Group with nil group",
			ve:   &VisibilityExpressionGroup{Group: nil},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rm.Embed(tc.ve)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}

	rm.Close()
}

func TestVisibilityExpressionIsVisible(t *testing.T) {
	layer1 := &Group{Name: "Layer1"}
	layer2 := &Group{Name: "Layer2"}
	layer3 := &Group{Name: "Layer3"}

	tests := []struct {
		name     string
		ve       VisibilityExpression
		states   map[*Group]bool
		expected bool
	}{
		{
			name:     "group on",
			ve:       &VisibilityExpressionGroup{Group: layer1},
			states:   map[*Group]bool{layer1: true},
			expected: true,
		},
		{
			name:     "group off",
			ve:       &VisibilityExpressionGroup{Group: layer1},
			states:   map[*Group]bool{layer1: false},
			expected: false,
		},
		{
			name: "And all on",
			ve: &VisibilityExpressionAnd{
				Args: []VisibilityExpression{
					&VisibilityExpressionGroup{Group: layer1},
					&VisibilityExpressionGroup{Group: layer2},
				},
			},
			states:   map[*Group]bool{layer1: true, layer2: true},
			expected: true,
		},
		{
			name: "And one off",
			ve: &VisibilityExpressionAnd{
				Args: []VisibilityExpression{
					&VisibilityExpressionGroup{Group: layer1},
					&VisibilityExpressionGroup{Group: layer2},
				},
			},
			states:   map[*Group]bool{layer1: true, layer2: false},
			expected: false,
		},
		{
			name: "Or all off",
			ve: &VisibilityExpressionOr{
				Args: []VisibilityExpression{
					&VisibilityExpressionGroup{Group: layer1},
					&VisibilityExpressionGroup{Group: layer2},
				},
			},
			states:   map[*Group]bool{layer1: false, layer2: false},
			expected: false,
		},
		{
			name: "Or one on",
			ve: &VisibilityExpressionOr{
				Args: []VisibilityExpression{
					&VisibilityExpressionGroup{Group: layer1},
					&VisibilityExpressionGroup{Group: layer2},
				},
			},
			states:   map[*Group]bool{layer1: false, layer2: true},
			expected: true,
		},
		{
			name: "Not on",
			ve: &VisibilityExpressionNot{
				Arg: &VisibilityExpressionGroup{Group: layer1},
			},
			states:   map[*Group]bool{layer1: true},
			expected: false,
		},
		{
			name: "Not off",
			ve: &VisibilityExpressionNot{
				Arg: &VisibilityExpressionGroup{Group: layer1},
			},
			states:   map[*Group]bool{layer1: false},
			expected: true,
		},
		{
			name: "complex: OCG1 OR (NOT OCG2) OR (OCG3 AND OCG4)",
			ve: &VisibilityExpressionOr{
				Args: []VisibilityExpression{
					&VisibilityExpressionGroup{Group: layer1},
					&VisibilityExpressionNot{
						Arg: &VisibilityExpressionGroup{Group: layer2},
					},
					&VisibilityExpressionAnd{
						Args: []VisibilityExpression{
							&VisibilityExpressionGroup{Group: layer3},
						},
					},
				},
			},
			states:   map[*Group]bool{layer1: false, layer2: true, layer3: false},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.ve.isVisible(tc.states)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func FuzzVisibilityExpression(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, tc := range visibilityExpressionTestCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		rm := pdf.NewResourceManager(w)
		obj, err := rm.Embed(tc.ve)
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
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		objPDF := r.GetMeta().Trailer["Quir:E"]
		if objPDF == nil {
			t.Skip("missing PDF object")
		}

		extractor := pdf.NewExtractor(r)
		ve, err := ExtractVisibilityExpression(extractor, objPDF)
		if err != nil {
			t.Skip("malformed visibility expression")
		}

		testVisibilityExpressionRoundTrip(t, pdf.GetVersion(r), ve)
	})
}
