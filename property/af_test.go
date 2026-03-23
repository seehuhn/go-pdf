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
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/optional"
)

func TestAFAsDirectDict(t *testing.T) {
	af := &AF{
		AssociatedFiles: []*file.Specification{
			{FileName: "test.txt"},
		},
	}
	if af.AsDirectDict() != nil {
		t.Error("AF.AsDirectDict() should return nil")
	}
}

func TestAFEmptyError(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	af := &AF{
		AssociatedFiles: []*file.Specification{},
	}

	_, err := rm.Embed(af)
	if err == nil {
		t.Error("Embed() with empty AssociatedFiles should return error")
	}
}

func TestAFRoundTripSingleUse(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	original := &AF{
		MCID: optional.NewUInt(99),
		AssociatedFiles: []*file.Specification{
			{
				FileName:       "report.xml",
				AFRelationship: file.RelationshipData,
			},
		},
		SingleUse: true,
	}

	embedded, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("Embed() failed: %v", err)
	}

	// SingleUse should produce a direct dict
	if _, ok := embedded.(pdf.Dict); !ok {
		t.Errorf("Embed() with SingleUse=true returned %T, want pdf.Dict", embedded)
	}

	x := pdf.NewExtractor(w)
	decoded, err := ExtractList(x, nil, embedded, true)
	if err != nil {
		t.Fatalf("ExtractList() failed: %v", err)
	}

	// AF dicts contain indirect references, so AsDirectDict returns nil
	if decoded.AsDirectDict() != nil {
		t.Error("decoded.AsDirectDict() should be nil for AF (contains indirect refs)")
	}

	// re-embed and extract to verify round-trip equality
	w2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm2 := pdf.NewResourceManager(w2)
	embedded2, err := rm2.Embed(decoded)
	if err != nil {
		t.Fatalf("re-Embed() failed: %v", err)
	}
	x2 := pdf.NewExtractor(w2)
	decoded2, err := ExtractList(x2, nil, embedded2, true)
	if err != nil {
		t.Fatalf("second ExtractList() failed: %v", err)
	}
	if !ListsEqual(decoded, decoded2) {
		t.Error("round trip failed: lists not equal")
	}
}

func TestAFRoundTripIndirect(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	original := &AF{
		MCID: optional.NewUInt(42),
		AssociatedFiles: []*file.Specification{
			{
				FileName:       "data1.csv",
				AFRelationship: file.RelationshipData,
			},
			{
				FileName:       "data2.csv",
				AFRelationship: file.RelationshipData,
			},
		},
		SingleUse: false,
	}

	embedded, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("Embed() failed: %v", err)
	}

	// SingleUse=false should produce a reference
	if _, ok := embedded.(pdf.Reference); !ok {
		t.Errorf("Embed() with SingleUse=false returned %T, want pdf.Reference", embedded)
	}

	x := pdf.NewExtractor(w)
	decoded, err := ExtractList(x, nil, embedded, false)
	if err != nil {
		t.Fatalf("ExtractList() failed: %v", err)
	}

	// indirect list should return nil from AsDirectDict
	if decoded.AsDirectDict() != nil {
		t.Error("decoded.AsDirectDict() should be nil for indirect list")
	}

	// re-embed and extract to verify round-trip equality
	w2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm2 := pdf.NewResourceManager(w2)
	embedded2, err := rm2.Embed(decoded)
	if err != nil {
		t.Fatalf("re-Embed() failed: %v", err)
	}
	x2 := pdf.NewExtractor(w2)
	decoded2, err := ExtractList(x2, nil, embedded2, false)
	if err != nil {
		t.Fatalf("second ExtractList() failed: %v", err)
	}
	if !ListsEqual(decoded, decoded2) {
		t.Error("round trip failed: lists not equal")
	}
}

func TestAFWithoutMCID(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	original := &AF{
		AssociatedFiles: []*file.Specification{
			{FileName: "test.txt"},
		},
		SingleUse: true,
	}

	embedded, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("Embed() failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := ExtractList(x, nil, embedded, true)
	if err != nil {
		t.Fatalf("ExtractList() failed: %v", err)
	}

	// AF dicts contain indirect references, so AsDirectDict returns nil
	if decoded.AsDirectDict() != nil {
		t.Error("decoded.AsDirectDict() should be nil for AF (contains indirect refs)")
	}

	// re-embed and extract to verify round-trip equality
	w2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm2 := pdf.NewResourceManager(w2)
	embedded2, err := rm2.Embed(decoded)
	if err != nil {
		t.Fatalf("re-Embed() failed: %v", err)
	}
	x2 := pdf.NewExtractor(w2)
	decoded2, err := ExtractList(x2, nil, embedded2, true)
	if err != nil {
		t.Fatalf("second ExtractList() failed: %v", err)
	}
	if !ListsEqual(decoded, decoded2) {
		t.Error("round trip failed: lists not equal")
	}
}
