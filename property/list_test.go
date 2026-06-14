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

package property

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

type testCase struct {
	Name       string
	Dict       pdf.Dict
	IsIndirect bool
}

var testCases = []testCase{
	{
		Name: "Empty",
		Dict: pdf.Dict{},
	},
	{
		Name: "MCID",
		Dict: pdf.Dict{
			"MCID": pdf.Integer(42),
		},
	},
	{
		Name: "AF-MCAF",
		Dict: pdf.Dict{
			"MCAF": pdf.Array{
				pdf.Dict{"F": pdf.String("file1.txt")},
				pdf.Dict{"F": pdf.String("file2.txt")},
			},
		},
	},
	{
		Name: "Artifact-Simple",
		Dict: pdf.Dict{
			"Type": pdf.Name("Pagination"),
		},
	},
	{
		Name: "Artifact-Full",
		Dict: pdf.Dict{
			"Type":     pdf.Name("Layout"),
			"BBox":     &pdf.Rectangle{LLx: 10, LLy: 20, URx: 100, URy: 200},
			"Attached": pdf.Array{pdf.Name("Top"), pdf.Name("Bottom")},
			"Subtype":  pdf.Name("Header"),
		},
	},
	{
		Name: "Span-Text",
		Dict: pdf.Dict{
			"Alt":        pdf.TextString("Alternative text"),
			"ActualText": pdf.TextString("Actual text"),
			"E":          pdf.TextString("Expansion"),
			"Lang":       pdf.TextString("en-US"),
		},
	},
	{
		Name: "OC-OCG",
		Dict: pdf.Dict{
			"Type": pdf.Name("OCG"),
			"Name": pdf.TextString("Layer 1"),
		},
	},
	{
		Name: "Mixed-Values",
		Dict: pdf.Dict{
			"String": pdf.String("test"),
			"Name":   pdf.Name("TestName"),
			"Int":    pdf.Integer(123),
			"Real":   pdf.Real(3.14),
			"Bool":   pdf.Boolean(true),
			"Array":  pdf.Array{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)},
		},
	},
	{
		Name: "Indirect",
		Dict: pdf.Dict{
			"MCID": pdf.Integer(99),
		},
		IsIndirect: true,
	},
}

func testRoundTrip(t *testing.T, dict pdf.Dict, isIndirect bool) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	// create property list
	var obj pdf.Object = dict
	if isIndirect {
		ref := w.Alloc()
		err := w.Put(ref, dict)
		if err != nil {
			t.Fatalf("put failed: %v", err)
		}
		obj = ref
	}

	x := pdf.NewExtractor(w)
	original, err := ExtractList(x, nil, obj, !isIndirect)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	// embed
	embedded, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	// extract again
	decoded, err := ExtractList(x, nil, embedded, !isIndirect)
	if err != nil {
		t.Fatalf("second extract failed: %v", err)
	}

	// compare using ListsEqual
	if !ListsEqual(original, decoded) {
		t.Error("round trip failed: lists not equal")
	}
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			testRoundTrip(t, tc.Dict, tc.IsIndirect)
		})
	}
}

func FuzzRoundTrip(f *testing.F) {
	// seed corpus with test cases
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(pdf.V2_0, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		var obj pdf.Object = tc.Dict
		if tc.IsIndirect {
			ref := w.Alloc()
			err := w.Put(ref, tc.Dict)
			if err != nil {
				continue
			}
			obj = ref
		}

		w.GetMeta().Trailer["Quir:E"] = obj
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing property list object")
		}

		x := pdf.NewExtractor(r)
		propList, err := pdf.ExtractorGet(x, nil, obj, ExtractList)
		if err != nil {
			t.Skip("malformed property list")
		}
		if propList == nil {
			t.Skip("no property list")
		}

		// check that AsDirectDict doesn't crash
		_ = propList.AsDirectDict()

		// round-trip test
		w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
		rm := pdf.NewResourceManager(w)

		embedded, err := rm.Embed(propList)
		if err != nil {
			t.Fatalf("embed failed: %v", err)
		}

		err = rm.Close()
		if err != nil {
			t.Fatalf("rm.Close failed: %v", err)
		}

		err = w.Close()
		if err != nil {
			t.Fatalf("w.Close failed: %v", err)
		}

		// use the writer as a getter
		x2 := pdf.NewExtractor(w)
		decoded, err := pdf.ExtractorGet(x2, nil, embedded, ExtractList)
		if err != nil {
			t.Fatalf("second extract failed: %v", err)
		}

		// verify round-trip equality
		if !ListsEqual(propList, decoded) {
			t.Error("round trip failed: lists not equal")
		}
	})
}
