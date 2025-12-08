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

package collection

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var testCases = []struct {
	name    string
	version pdf.Version
	item    *ItemDict
}{
	{
		name:    "simple_string_values",
		version: pdf.V1_7,
		item: &ItemDict{
			Data: map[pdf.Name]ItemValue{
				"Title":  {Val: "Document Title"},
				"Author": {Val: "John Doe"},
			},
		},
	},
	{
		name:    "mixed_types",
		version: pdf.V1_7,
		item: &ItemDict{
			Data: map[pdf.Name]ItemValue{
				"Name":     {Val: "Test Document"},
				"Size":     {Val: int64(1024)},
				"Rating":   {Val: float64(4.5)},
				"Modified": {Val: time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)},
			},
		},
	},
	{
		name:    "with_prefixes",
		version: pdf.V1_7,
		item: &ItemDict{
			Data: map[pdf.Name]ItemValue{
				"Size":     {Val: int64(1048576), Prefix: "Size: "},
				"Category": {Val: "PDF Document", Prefix: "Type: "},
			},
		},
	},
	{
		name:    "mixed_with_and_without_prefixes",
		version: pdf.V2_0,
		item: &ItemDict{
			Data: map[pdf.Name]ItemValue{
				"Title":    {Val: "My Document"},
				"Size":     {Val: int64(2048), Prefix: "File size: "},
				"Created":  {Val: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
				"Priority": {Val: float64(7.8), Prefix: "Priority: "},
			},
		},
	},
	{
		name:    "single_use_enabled",
		version: pdf.V1_7,
		item: &ItemDict{
			Data: map[pdf.Name]ItemValue{
				"Key": {Val: "Value"},
			},
			SingleUse: true,
		},
	},
	{
		name:    "date_values",
		version: pdf.V2_0,
		item: &ItemDict{
			Data: map[pdf.Name]ItemValue{
				"Created":  {Val: time.Date(2023, 6, 15, 14, 30, 45, 0, time.UTC)},
				"Modified": {Val: time.Date(2024, 12, 1, 9, 15, 30, 0, time.UTC), Prefix: "Last modified: "},
			},
		},
	},
	{
		name:    "numeric_values",
		version: pdf.V1_7,
		item: &ItemDict{
			Data: map[pdf.Name]ItemValue{
				"Count":      {Val: int64(42)},
				"Percentage": {Val: float64(87.5)},
				"Score":      {Val: int64(100), Prefix: "Score: "},
				"Average":    {Val: float64(3.14159), Prefix: "Avg: "},
			},
		},
	},
}

func TestItemDictRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripTest(t, tc.version, tc.item)
		})
	}
}

func roundTripTest(t *testing.T, version pdf.Version, item1 *ItemDict) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	// Embed the item
	obj, err := rm.Embed(item1)
	if err != nil {
		t.Fatal(err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Store in trailer for extraction
	w.GetMeta().Trailer["Quir:E"] = obj
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Extract the item
	x := pdf.NewExtractor(w)
	objFromTrailer := w.GetMeta().Trailer["Quir:E"]
	if objFromTrailer == nil {
		t.Fatal("missing test object")
	}

	item2, err := ExtractItemDict(x, objFromTrailer)
	if err != nil {
		t.Fatal(err)
	}

	// SingleUse is inferred from how the object was stored (direct vs indirect)
	if diff := cmp.Diff(item1, item2); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestItemDictValidation(t *testing.T) {
	t.Run("type_key_forbidden", func(t *testing.T) {
		item := &ItemDict{
			Data: map[pdf.Name]ItemValue{
				"Type": {Val: "should not be allowed"},
				"Name": {Val: "Test"},
			},
		}

		w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
		rm := pdf.NewResourceManager(w)

		_, err := rm.Embed(item)
		if err == nil {
			t.Error("expected error for Type key, got nil")
		}
	})

	t.Run("invalid_value_type", func(t *testing.T) {
		item := &ItemDict{
			Data: map[pdf.Name]ItemValue{
				"Bad": {Val: []string{"not", "supported"}},
			},
		}

		w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
		rm := pdf.NewResourceManager(w)

		_, err := rm.Embed(item)
		if err == nil {
			t.Error("expected error for invalid value type, got nil")
		}
	})

	t.Run("version_requirement", func(t *testing.T) {
		item := &ItemDict{
			Data: map[pdf.Name]ItemValue{
				"Test": {Val: "value"},
			},
		}

		w, _ := memfile.NewPDFWriter(pdf.V1_6, nil)
		rm := pdf.NewResourceManager(w)

		_, err := rm.Embed(item)
		if err == nil {
			t.Error("expected version error, got nil")
		}
	})
}

func TestExtractItemDictMalformed(t *testing.T) {
	t.Run("missing_dictionary", func(t *testing.T) {
		w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
		x := pdf.NewExtractor(w)

		_, err := ExtractItemDict(x, nil)
		if err == nil {
			t.Error("expected error for missing dictionary, got nil")
		}
	})

	t.Run("wrong_type", func(t *testing.T) {
		w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
		x := pdf.NewExtractor(w)

		dict := pdf.Dict{
			"Type": pdf.Name("WrongType"),
			"Key":  pdf.TextString("value"),
		}

		_, err := ExtractItemDict(x, dict)
		if err == nil {
			t.Error("expected error for wrong type, got nil")
		}
	})
}

func FuzzItemDictRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)
		rm := pdf.NewResourceManager(w)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		obj, err := rm.Embed(tc.item)
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
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		item1, err := ExtractItemDict(x, objPDF)
		if err != nil {
			t.Skip("malformed collection item")
		}

		roundTripTest(t, r.GetMeta().Version, item1)
	})
}
