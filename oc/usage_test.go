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

func TestUsageRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		usage *Usage
	}{
		{
			name:  "empty",
			usage: &Usage{},
		},
		{
			name: "creator_info",
			usage: &Usage{
				Creator: &UsageCreator{
					Creator: "Test Application",
					Subtype: "Artwork",
					AdditionalInfo: pdf.Dict{
						"Version": pdf.TextString("1.0"),
					},
				},
			},
		},
		{
			name: "language",
			usage: &Usage{
				Language: &UsageLanguage{
					Lang:      language.MustParse("es-MX"),
					Preferred: true,
				},
			},
		},
		{
			name: "export",
			usage: &Usage{
				Export: &UsageExport{
					ExportState: true,
				},
			},
		},
		{
			name: "zoom",
			usage: &Usage{
				Zoom: &UsageZoom{
					Min: 1.0,
					Max: 10.0,
				},
			},
		},
		{
			name: "zoom_infinity",
			usage: &Usage{
				Zoom: &UsageZoom{
					Min: 2.0,
					Max: 1e308,
				},
			},
		},
		{
			name: "print",
			usage: &Usage{
				Print: &UsagePrint{
					Subtype:    PrintSubtypeWatermark,
					PrintState: true,
				},
			},
		},
		{
			name: "view",
			usage: &Usage{
				View: &UsageView{
					ViewState: false,
				},
			},
		},
		{
			name: "user_single",
			usage: &Usage{
				User: &UsageUser{
					Type: UserTypeIndividual,
					Name: []string{"John Doe"},
				},
			},
		},
		{
			name: "user_multiple",
			usage: &Usage{
				User: &UsageUser{
					Type: UserTypeOrganisation,
					Name: []string{"Company A", "Company B"},
				},
			},
		},
		{
			name: "page_element",
			usage: &Usage{
				PageElement: &UsagePageElement{
					Subtype: PageElementHeaderFooter,
				},
			},
		},
		{
			name: "complex",
			usage: &Usage{
				Creator: &UsageCreator{
					Creator: "PDF Editor Pro",
					Subtype: "Technical",
				},
				Language: &UsageLanguage{
					Lang:      language.English,
					Preferred: false,
				},
				Export: &UsageExport{
					ExportState: false,
				},
				Zoom: &UsageZoom{
					Min: 0.5,
					Max: 20.0,
				},
				Print: &UsagePrint{
					Subtype:    PrintSubtypePrintersMarks,
					PrintState: true,
				},
				View: &UsageView{
					ViewState: true,
				},
				User: &UsageUser{
					Type: UserTypeTitle,
					Name: []string{"Manager", "Director"},
				},
				PageElement: &UsagePageElement{
					Subtype: PageElementBackground,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// test with SingleUse = false (indirect reference)
			tt.usage.SingleUse = false
			testUsageRoundTrip(t, tt.usage, "indirect")

			// test with SingleUse = true (direct dictionary)
			tt.usage.SingleUse = true
			testUsageRoundTrip(t, tt.usage, "direct")
		})
	}
}

func testUsageRoundTrip(t *testing.T, original *Usage, mode string) {
	buf, _ := memfile.NewPDFWriter(pdf.V1_0, nil)

	rm := pdf.NewResourceManager(buf)

	// embed the usage dictionary
	obj, _, err2 := original.Embed(rm)
	if err2 != nil {
		t.Fatalf("%s: embed: %v", mode, err2)
	}

	// for indirect references, store the object
	if ref, ok := obj.(pdf.Reference); ok {
		// object is already stored via Embed
		obj = ref
	}

	err2 = rm.Close()
	if err2 != nil {
		t.Fatalf("%s: close writer: %v", mode, err2)
	}

	// extract the usage dictionary
	extractor := pdf.NewExtractor(buf)
	extracted, err3 := ExtractUsage(extractor, obj)
	if err3 != nil {
		t.Fatalf("%s: extract: %v", mode, err3)
	}

	// normalize for comparison
	normalizeUsage(original)
	normalizeUsage(extracted)

	// compare
	opts := []cmp.Option{
		cmp.AllowUnexported(Usage{}),
		cmp.Comparer(func(a, b language.Tag) bool {
			return a.String() == b.String()
		}),
	}
	if diff := cmp.Diff(original, extracted, opts...); diff != "" {
		t.Errorf("%s: round trip failed (-want +got):\n%s", mode, diff)
	}
}

func normalizeUsage(u *Usage) {
	// normalize language tags to their canonical form
	if u.Language != nil && u.Language.Lang != language.Und {
		// parse and re-format to get canonical form
		canonical, err := language.Parse(u.Language.Lang.String())
		if err == nil {
			u.Language.Lang = canonical
		}
	}

	// normalize zoom max value for infinity
	if u.Zoom != nil && u.Zoom.Max >= 1e307 {
		u.Zoom.Max = 1e308
	}

	// clear AdditionalInfo if empty
	if u.Creator != nil && len(u.Creator.AdditionalInfo) == 0 {
		u.Creator.AdditionalInfo = nil
	}

	// normalize AdditionalInfo types (TextString -> String during PDF processing)
	if u.Creator != nil && u.Creator.AdditionalInfo != nil {
		for key, val := range u.Creator.AdditionalInfo {
			if ts, ok := val.(pdf.TextString); ok {
				u.Creator.AdditionalInfo[key] = pdf.String(ts)
			}
		}
	}
}

func TestUsageValidation(t *testing.T) {
	buf, _ := memfile.NewPDFWriter(pdf.V1_0, nil)
	rm := pdf.NewResourceManager(buf)

	// Test invalid Zoom constraint: Min > Max
	usage := &Usage{
		Zoom: &UsageZoom{
			Min: 10.0,
			Max: 5.0, // Max < Min should fail
		},
	}

	_, _, err := usage.Embed(rm)
	if err == nil {
		t.Error("expected error for Zoom.Min > Zoom.Max, but got none")
	}
	if err.Error() != "Zoom.Min must be less than or equal to Zoom.Max" {
		t.Errorf("unexpected error message: %v", err)
	}
}
