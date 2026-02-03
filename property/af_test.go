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

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/optional"
)

func TestAFIsDirect(t *testing.T) {
	af := &AF{
		AssociatedFiles: []*file.Specification{
			{FileName: "test.txt"},
		},
	}
	if af.IsDirect() {
		t.Error("AF.IsDirect() should return false")
	}
}

func TestAFKeys(t *testing.T) {
	tests := []struct {
		name string
		af   *AF
		want []pdf.Name
	}{
		{
			name: "only MCAF",
			af: &AF{
				AssociatedFiles: []*file.Specification{{FileName: "test.txt"}},
			},
			want: []pdf.Name{"MCAF"},
		},
		{
			name: "MCAF and MCID",
			af: &AF{
				MCID:            optional.NewUInt(42),
				AssociatedFiles: []*file.Specification{{FileName: "test.txt"}},
			},
			want: []pdf.Name{"MCAF", "MCID"},
		},
		{
			name: "empty AssociatedFiles",
			af: &AF{
				MCID: optional.NewUInt(42),
			},
			want: []pdf.Name{"MCAF", "MCID"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.af.Keys()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Keys() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAFGet(t *testing.T) {
	af := &AF{
		MCID: optional.NewUInt(42),
		AssociatedFiles: []*file.Specification{
			{FileName: "test1.txt"},
			{FileName: "test2.txt"},
		},
	}

	// test getting MCID
	val, err := af.Get("MCID")
	if err != nil {
		t.Fatalf("Get(MCID) failed: %v", err)
	}
	if val.(pdf.Integer) != 42 {
		t.Errorf("Get(MCID) = %v, want 42", val)
	}

	// test getting MCAF - should return array of file spec references
	mcafVal, err := af.Get("MCAF")
	if err != nil {
		t.Fatalf("Get(MCAF) failed: %v", err)
	}
	arr, ok := mcafVal.AsPDF(0).(pdf.Array)
	if !ok {
		t.Fatalf("Get(MCAF) returned %T, want pdf.Array", mcafVal.AsPDF(0))
	}
	if len(arr) != 2 {
		t.Errorf("Get(MCAF) array length = %d, want 2", len(arr))
	}

	// test getting non-existent key
	_, err = af.Get("NonExistent")
	if err != ErrNoKey {
		t.Errorf("Get(NonExistent) error = %v, want ErrNoKey", err)
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
	decoded, err := ExtractList(x, embedded)
	if err != nil {
		t.Fatalf("ExtractList() failed: %v", err)
	}

	// verify keys
	origKeys := original.Keys()
	decodedKeys := decoded.Keys()
	if diff := cmp.Diff(origKeys, decodedKeys); diff != "" {
		t.Errorf("keys mismatch (-want +got):\n%s", diff)
	}

	// verify MCID
	mcidVal, err := decoded.Get("MCID")
	if err != nil {
		t.Fatalf("Get(MCID) failed: %v", err)
	}
	mcidInt := mcidVal.AsPDF(0).(pdf.Integer)
	if mcidInt != 99 {
		t.Errorf("MCID = %d, want 99", mcidInt)
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
	decoded, err := ExtractList(x, embedded)
	if err != nil {
		t.Fatalf("ExtractList() failed: %v", err)
	}

	// verify keys
	origKeys := original.Keys()
	decodedKeys := decoded.Keys()
	if diff := cmp.Diff(origKeys, decodedKeys); diff != "" {
		t.Errorf("keys mismatch (-want +got):\n%s", diff)
	}

	// verify MCAF array length
	mcafVal, err := decoded.Get("MCAF")
	if err != nil {
		t.Fatalf("Get(MCAF) failed: %v", err)
	}

	// resolve the array through the extractor
	mcafResolved := mcafVal.AsPDF(0)
	arr, ok := mcafResolved.(pdf.Array)
	if !ok {
		t.Fatalf("MCAF resolved to %T, want pdf.Array", mcafResolved)
	}
	if len(arr) != 2 {
		t.Errorf("MCAF array length = %d, want 2", len(arr))
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
	decoded, err := ExtractList(x, embedded)
	if err != nil {
		t.Fatalf("ExtractList() failed: %v", err)
	}

	// should only have MCAF key
	keys := decoded.Keys()
	if diff := cmp.Diff([]pdf.Name{"MCAF"}, keys); diff != "" {
		t.Errorf("keys mismatch (-want +got):\n%s", diff)
	}

	// MCID should not be present
	_, err = decoded.Get("MCID")
	if err != ErrNoKey {
		t.Errorf("Get(MCID) error = %v, want ErrNoKey", err)
	}
}
