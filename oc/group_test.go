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

package oc

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestGroupRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		group *Group
	}{
		{
			name: "minimal",
			group: &Group{
				Name: "Test Group",
			},
		},
		{
			name: "with_single_intent",
			group: &Group{
				Name:   "Design Layer",
				Intent: []pdf.Name{"Design"},
			},
		},
		{
			name: "with_multiple_intents",
			group: &Group{
				Name:   "Multi Purpose Layer",
				Intent: []pdf.Name{"View", "Design"},
			},
		},
		{
			name: "with_usage",
			group: &Group{
				Name: "Language Layer",
				Usage: &Usage{
					Language: &UsageLanguage{
						Lang:      language.English,
						Preferred: true,
					},
				},
			},
		},
		{
			name: "complex",
			group: &Group{
				Name:   "Complex Layer",
				Intent: []pdf.Name{"View", "Print"},
				Usage: &Usage{
					Creator: &UsageCreator{
						Creator: "Test App",
						Subtype: "Artwork",
					},
					Language: &UsageLanguage{
						Lang:      language.MustParse("es-MX"),
						Preferred: false,
					},
					Zoom: &UsageZoom{
						Min: 1.0,
						Max: 10.0,
					},
					Print: &UsagePrint{
						Subtype:    PrintSubtypeWatermark,
						PrintState: true,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testGroupRoundTrip(t, tt.group)
		})
	}
}

func testGroupRoundTrip(t *testing.T, original *Group) {
	buf, _ := memfile.NewPDFWriter(pdf.V1_0, nil)
	rm := pdf.NewResourceManager(buf)

	// embed the group
	obj, _, err := original.Embed(rm)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	// verify it's an indirect reference
	ref, ok := obj.(pdf.Reference)
	if !ok {
		t.Fatal("expected Group.Embed to return pdf.Reference")
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("close writer: %v", err)
	}

	// extract the group
	extractor := pdf.NewExtractor(buf)
	extracted, err := ExtractGroup(extractor, ref)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	// normalize for comparison
	normalizeGroup(original)
	normalizeGroup(extracted)

	// compare
	opts := []cmp.Option{
		cmp.AllowUnexported(Group{}, Usage{}),
		cmp.Comparer(func(a, b language.Tag) bool {
			return a.String() == b.String()
		}),
	}
	if diff := cmp.Diff(original, extracted, opts...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func normalizeGroup(g *Group) {
	// normalize default Intent
	if len(g.Intent) == 0 || (len(g.Intent) == 1 && g.Intent[0] == "View") {
		g.Intent = []pdf.Name{"View"}
	}

	// normalize usage if present
	if g.Usage != nil {
		normalizeUsage(g.Usage)
	}
}

func TestGroupValidation(t *testing.T) {
	buf, _ := memfile.NewPDFWriter(pdf.V1_0, nil)
	rm := pdf.NewResourceManager(buf)

	// Test empty name should fail
	group := &Group{
		Name: "",
	}

	_, _, err := group.Embed(rm)
	if err == nil {
		t.Error("expected error for empty Group.Name, but got none")
	}
	if err.Error() != "Group.Name is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGroupIntentHandling(t *testing.T) {
	tests := []struct {
		name           string
		inputIntent    []pdf.Name
		expectedIntent []pdf.Name
	}{
		{
			name:           "empty_intent_uses_default",
			inputIntent:    nil,
			expectedIntent: []pdf.Name{"View"},
		},
		{
			name:           "single_intent",
			inputIntent:    []pdf.Name{"Design"},
			expectedIntent: []pdf.Name{"Design"},
		},
		{
			name:           "multiple_intents",
			inputIntent:    []pdf.Name{"View", "Design", "Print"},
			expectedIntent: []pdf.Name{"View", "Design", "Print"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := &Group{
				Name:   "Test Group",
				Intent: tt.inputIntent,
			}

			testGroupRoundTrip(t, group)

			// verify the intent was set correctly
			if len(group.Intent) != len(tt.expectedIntent) {
				t.Errorf("expected %d intents, got %d", len(tt.expectedIntent), len(group.Intent))
			}
			for i, expected := range tt.expectedIntent {
				if i >= len(group.Intent) || group.Intent[i] != expected {
					t.Errorf("expected intent[%d] = %q, got %q", i, expected, group.Intent[i])
				}
			}
		})
	}
}
