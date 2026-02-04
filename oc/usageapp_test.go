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

var (
	usageAppGroup1 = &Group{Name: "Group 1", Intent: []pdf.Name{"View"}}
	usageAppGroup2 = &Group{Name: "Group 2", Intent: []pdf.Name{"Design"}}

	usageAppTestCases = []struct {
		name    string
		version pdf.Version
		data    *UsageApplication
	}{
		{
			name:    "view_single_category/indirect",
			version: pdf.V1_5,
			data: &UsageApplication{
				Event:    EventView,
				Category: []Category{CategoryView},
			},
		},
		{
			name:    "view_single_category/direct",
			version: pdf.V1_5,
			data: &UsageApplication{
				SingleUse: true,
				Event:     EventView,
				Category:  []Category{CategoryView},
			},
		},
		{
			name:    "print_single_category/indirect",
			version: pdf.V1_5,
			data: &UsageApplication{
				Event:    EventPrint,
				Category: []Category{CategoryPrint},
			},
		},
		{
			name:    "print_single_category/direct",
			version: pdf.V1_5,
			data: &UsageApplication{
				SingleUse: true,
				Event:     EventPrint,
				Category:  []Category{CategoryPrint},
			},
		},
		{
			name:    "export_single_category/indirect",
			version: pdf.V1_5,
			data: &UsageApplication{
				Event:    EventExport,
				Category: []Category{CategoryExport},
			},
		},
		{
			name:    "export_single_category/direct",
			version: pdf.V1_5,
			data: &UsageApplication{
				SingleUse: true,
				Event:     EventExport,
				Category:  []Category{CategoryExport},
			},
		},
		{
			name:    "multiple_categories/indirect",
			version: pdf.V1_5,
			data: &UsageApplication{
				Event:    EventPrint,
				Category: []Category{CategoryPrint, CategoryZoom, CategoryLanguage},
			},
		},
		{
			name:    "multiple_categories/direct",
			version: pdf.V1_5,
			data: &UsageApplication{
				SingleUse: true,
				Event:     EventPrint,
				Category:  []Category{CategoryPrint, CategoryZoom, CategoryLanguage},
			},
		},
		{
			name:    "all_categories/indirect",
			version: pdf.V1_5,
			data: &UsageApplication{
				Event: EventView,
				Category: []Category{
					CategoryCreatorInfo, CategoryLanguage, CategoryExport, CategoryZoom,
					CategoryPrint, CategoryView, CategoryUser, CategoryPageElement,
				},
			},
		},
		{
			name:    "all_categories/direct",
			version: pdf.V1_5,
			data: &UsageApplication{
				SingleUse: true,
				Event:     EventView,
				Category: []Category{
					CategoryCreatorInfo, CategoryLanguage, CategoryExport, CategoryZoom,
					CategoryPrint, CategoryView, CategoryUser, CategoryPageElement,
				},
			},
		},
		{
			name:    "single_ocg/indirect",
			version: pdf.V1_5,
			data: &UsageApplication{
				Event:    EventPrint,
				OCGs:     []*Group{usageAppGroup1},
				Category: []Category{CategoryPrint},
			},
		},
		{
			name:    "single_ocg/direct",
			version: pdf.V1_5,
			data: &UsageApplication{
				SingleUse: true,
				Event:     EventPrint,
				OCGs:      []*Group{usageAppGroup1},
				Category:  []Category{CategoryPrint},
			},
		},
		{
			name:    "multiple_ocgs/indirect",
			version: pdf.V1_5,
			data: &UsageApplication{
				Event:    EventExport,
				OCGs:     []*Group{usageAppGroup1, usageAppGroup2},
				Category: []Category{CategoryExport, CategoryZoom},
			},
		},
		{
			name:    "multiple_ocgs/direct",
			version: pdf.V1_5,
			data: &UsageApplication{
				SingleUse: true,
				Event:     EventExport,
				OCGs:      []*Group{usageAppGroup1, usageAppGroup2},
				Category:  []Category{CategoryExport, CategoryZoom},
			},
		},
	}
)

func TestUsageAppValidation(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_5, nil)
	rm := pdf.NewResourceManager(w)

	// test invalid Event
	ua := &UsageApplication{
		Event:    Event("Invalid"),
		Category: []Category{CategoryView},
	}
	_, err := rm.Embed(ua)
	if err == nil {
		t.Error("expected error for invalid Event, but got none")
	}

	// test empty Category
	ua = &UsageApplication{
		Event:    EventView,
		Category: nil,
	}
	_, err = rm.Embed(ua)
	if err == nil {
		t.Error("expected error for empty Category, but got none")
	}

	// test invalid Category
	ua = &UsageApplication{
		Event:    EventView,
		Category: []Category{Category("Invalid")},
	}
	_, err = rm.Embed(ua)
	if err == nil {
		t.Error("expected error for invalid Category, but got none")
	}

	// test version check (PDF 1.4 should fail)
	w14, _ := memfile.NewPDFWriter(pdf.V1_4, nil)
	rm14 := pdf.NewResourceManager(w14)
	ua = &UsageApplication{
		Event:    EventView,
		Category: []Category{CategoryView},
	}
	_, err = rm14.Embed(ua)
	if err == nil {
		t.Error("expected version error for PDF 1.4, but got none")
	}
}

func TestUsageAppRoundTrip(t *testing.T) {
	for _, tc := range usageAppTestCases {
		t.Run(tc.name, func(t *testing.T) {
			testUsageAppRoundTrip(t, tc.version, tc.data)
		})
	}
}

func testUsageAppRoundTrip(t *testing.T, version pdf.Version, data *UsageApplication) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	obj, err := rm.Embed(data)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("embed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("rm.Close: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("w.Close: %v", err)
	}

	extractor := pdf.NewExtractor(w)
	extracted, err := pdf.ExtractorGet(extractor, obj, ExtractUsageApplication)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	opts := []cmp.Option{
		cmp.AllowUnexported(UsageApplication{}),
		cmp.Comparer(func(a, b *Group) bool {
			if a == nil && b == nil {
				return true
			}
			if a == nil || b == nil {
				return false
			}
			return a.Name == b.Name
		}),
	}
	if diff := cmp.Diff(data, extracted, opts...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func FuzzUsageAppRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, tc := range usageAppTestCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		rm := pdf.NewResourceManager(w)
		obj, err := rm.Embed(tc.data)
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

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		data, err := pdf.ExtractorGet(x, obj, ExtractUsageApplication)
		if err != nil {
			t.Skip("malformed object")
		}

		testUsageAppRoundTrip(t, pdf.GetVersion(r), data)
	})
}
