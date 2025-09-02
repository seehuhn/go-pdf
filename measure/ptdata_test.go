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

package measure

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestPtDataRoundTrip(t *testing.T) {
	original := &PtData{
		Subtype: PtDataSubtypeCloud,
		Names:   []string{PtDataNameLat, PtDataNameLon, PtDataNameAlt, "temperature", "sensor_id"},
		XPTS: [][]pdf.Object{
			{pdf.Number(40.7128), pdf.Number(-74.0060), pdf.Number(10.5), pdf.Number(22.3), pdf.String("NYC001")},
			{pdf.Number(40.7589), pdf.Number(-73.9851), pdf.Number(15.2), pdf.Number(21.8), pdf.String("NYC002")},
			{pdf.Number(40.7831), pdf.Number(-73.9712), pdf.Number(12.1), pdf.Number(23.1), pdf.String("NYC003")},
		},
		SingleUse: true,
	}

	// Test embedding
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	embedded, _, err := original.Embed(rm)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	// Test extraction
	extracted, err := ExtractPtData(w, embedded)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	// SingleUse is not stored in PDF, so reset it for comparison
	extracted.SingleUse = original.SingleUse

	if diff := cmp.Diff(extracted, original, cmp.AllowUnexported(PtData{})); diff != "" {
		t.Errorf("round trip failed (-got +want):\n%s", diff)
	}
}

func TestPtDataMinimal(t *testing.T) {
	original := &PtData{
		Subtype:   PtDataSubtypeCloud,
		Names:     []string{PtDataNameLat, PtDataNameLon},
		XPTS:      [][]pdf.Object{},
		SingleUse: true,
	}

	// Test round trip
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	embedded, _, err := original.Embed(rm)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	extracted, err := ExtractPtData(w, embedded)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	// SingleUse is not stored in PDF, so reset it for comparison
	extracted.SingleUse = original.SingleUse

	if diff := cmp.Diff(extracted, original, cmp.AllowUnexported(PtData{})); diff != "" {
		t.Errorf("round trip failed (-got +want):\n%s", diff)
	}
}

func TestPtDataIndirectObject(t *testing.T) {
	original := &PtData{
		Subtype: PtDataSubtypeCloud,
		Names:   []string{PtDataNameLat, PtDataNameLon},
		XPTS: [][]pdf.Object{
			{pdf.Real(40.7128), pdf.Real(-74.0060)}, // Use pdf.Real to match what PDF stores
		},
		SingleUse: false, // Should create indirect object
	}

	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	embedded, _, err := original.Embed(rm)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	// Should return a reference
	ref, ok := embedded.(pdf.Reference)
	if !ok {
		t.Errorf("expected reference, got %T", embedded)
	}

	// Extract using the reference
	extracted, err := ExtractPtData(w, ref)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	// SingleUse is not stored in PDF, so reset it for comparison
	extracted.SingleUse = original.SingleUse

	if diff := cmp.Diff(extracted, original, cmp.AllowUnexported(PtData{})); diff != "" {
		t.Errorf("round trip failed (-got +want):\n%s", diff)
	}
}
