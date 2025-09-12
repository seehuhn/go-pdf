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

package form

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/measure"
	"seehuhn.de/go/pdf/pieceinfo"
	"seehuhn.de/go/pdf/structure"
)

// testData implements pieceinfo.Data for testing
type testData struct {
	data string
}

func (t *testData) LastModified() time.Time {
	return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
}

func (t *testData) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	return pdf.String(t.data), pdf.Unused{}, nil
}

// TestRead verifies that a form XObject read from one PDF file can be written
// to another PDF file.
func TestRead(t *testing.T) {
	writer1, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm1 := pdf.NewResourceManager(writer1)

	form0 := &Form{
		Draw: func(w *graphics.Writer) error {
			w.SetFillColor(color.DeviceGray(0.5))
			w.Rectangle(0, 0, 100, 100)
			w.Fill()
			return nil
		},
		BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
	}
	ref, _, err := pdf.ResourceManagerEmbed(rm1, form0)
	if err != nil {
		t.Fatal(err)
	}
	err = rm1.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = writer1.Close()
	if err != nil {
		t.Fatal(err)
	}

	x1 := pdf.NewExtractor(writer1)
	form1, err := Extract(x1, ref)
	if err != nil {
		t.Fatal(err)
	}

	writer2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm2 := pdf.NewResourceManager(writer2)
	ref2, _, err := pdf.ResourceManagerEmbed(rm2, form1)
	if err != nil {
		t.Fatal(err)
	}
	err = rm2.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = writer2.Close()
	if err != nil {
		t.Fatal(err)
	}

	x2 := pdf.NewExtractor(writer2)
	form2, err := Extract(x2, ref2)
	if err != nil {
		t.Fatal(err)
	}

	// check that form1 and form2 are the same (excluding Draw function)
	if diff := cmp.Diff(form1, form2); diff != "" {
		t.Errorf("Round trip failed (-got +want):\n%s", diff)
	}
}

// TestReadWithPieceInfo verifies that PieceInfo is properly handled during
// form XObject read/write cycles.
func TestReadWithPieceInfo(t *testing.T) {
	writer1, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm1 := pdf.NewResourceManager(writer1)

	// create test PieceInfo with some data
	testPieceInfo := &pieceinfo.PieceInfo{
		Entries: map[pdf.Name]pieceinfo.Data{
			"TestApp": &testData{data: "test private data"},
		},
	}

	form0 := &Form{
		Draw: func(w *graphics.Writer) error {
			w.SetFillColor(color.DeviceGray(0.5))
			w.Rectangle(0, 0, 100, 100)
			w.Fill()
			return nil
		},
		BBox:         pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		PieceInfo:    testPieceInfo,
		LastModified: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	ref, _, err := pdf.ResourceManagerEmbed(rm1, form0)
	if err != nil {
		t.Fatal(err)
	}
	err = rm1.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = writer1.Close()
	if err != nil {
		t.Fatal(err)
	}

	x1 := pdf.NewExtractor(writer1)
	form1, err := Extract(x1, ref)
	if err != nil {
		t.Fatal(err)
	}

	// verify PieceInfo was preserved
	if form1.PieceInfo == nil {
		t.Error("PieceInfo was not preserved during extraction")
	}

	// test round-trip
	writer2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm2 := pdf.NewResourceManager(writer2)
	ref2, _, err := pdf.ResourceManagerEmbed(rm2, form1)
	if err != nil {
		t.Fatal(err)
	}
	err = rm2.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = writer2.Close()
	if err != nil {
		t.Fatal(err)
	}

	x2 := pdf.NewExtractor(writer2)
	form2, err := Extract(x2, ref2)
	if err != nil {
		t.Fatal(err)
	}

	// check that PieceInfo round-tripped correctly
	if form2.PieceInfo == nil {
		t.Error("PieceInfo was lost during round-trip")
	}
}

// TestPieceInfoRequiresLastModified verifies that LastModified is required
// when PieceInfo is present.
func TestPieceInfoRequiresLastModified(t *testing.T) {
	writer, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(writer)

	testPieceInfo := &pieceinfo.PieceInfo{
		Entries: map[pdf.Name]pieceinfo.Data{
			"TestApp": &testData{data: "test private data"},
		},
	}

	form := &Form{
		Draw: func(w *graphics.Writer) error {
			w.SetFillColor(color.DeviceGray(0.5))
			w.Rectangle(0, 0, 100, 100)
			w.Fill()
			return nil
		},
		BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		PieceInfo: testPieceInfo,
		// LastModified is intentionally not set to trigger validation error
	}

	_, _, err := pdf.ResourceManagerEmbed(rm, form)
	if err == nil {
		t.Error("Expected error when PieceInfo is present but LastModified is not set")
	}
}

// TestFormWithPtData verifies that PtData is properly handled during
// form XObject read/write cycles.
func TestFormWithPtData(t *testing.T) {
	writer1, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm1 := pdf.NewResourceManager(writer1)

	// create test PtData with some geospatial point data
	testPtData := &measure.PtData{
		Subtype: measure.PtDataSubtypeCloud,
		Names:   []string{measure.PtDataNameLat, measure.PtDataNameLon, measure.PtDataNameAlt},
		XPTS: [][]pdf.Object{
			{pdf.Number(40.7128), pdf.Number(-74.0060), pdf.Number(10.5)}, // NYC coordinates
			{pdf.Number(40.7589), pdf.Number(-73.9851), pdf.Number(15.2)}, // Central Park
		},
		SingleUse: false, // use as indirect object
	}

	form0 := &Form{
		Draw: func(w *graphics.Writer) error {
			w.SetFillColor(color.DeviceGray(0.5))
			w.Rectangle(0, 0, 100, 100)
			w.Fill()
			return nil
		},
		BBox:   pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		PtData: testPtData,
	}

	ref, _, err := pdf.ResourceManagerEmbed(rm1, form0)
	if err != nil {
		t.Fatal(err)
	}
	err = rm1.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = writer1.Close()
	if err != nil {
		t.Fatal(err)
	}

	x1 := pdf.NewExtractor(writer1)
	form1, err := Extract(x1, ref)
	if err != nil {
		t.Fatal(err)
	}

	// verify PtData was preserved
	if form1.PtData == nil {
		t.Error("PtData was not preserved during extraction")
		return
	}

	// check PtData content
	if form1.PtData.Subtype != measure.PtDataSubtypeCloud {
		t.Errorf("PtData subtype mismatch: got %s, want %s", form1.PtData.Subtype, measure.PtDataSubtypeCloud)
	}
	if len(form1.PtData.Names) != 3 {
		t.Errorf("PtData names length mismatch: got %d, want 3", len(form1.PtData.Names))
	}
	if len(form1.PtData.XPTS) != 2 {
		t.Errorf("PtData XPTS length mismatch: got %d, want 2", len(form1.PtData.XPTS))
	}

	// test round-trip
	writer2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm2 := pdf.NewResourceManager(writer2)
	ref2, _, err := pdf.ResourceManagerEmbed(rm2, form1)
	if err != nil {
		t.Fatal(err)
	}
	err = rm2.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = writer2.Close()
	if err != nil {
		t.Fatal(err)
	}

	x2 := pdf.NewExtractor(writer2)
	form2, err := Extract(x2, ref2)
	if err != nil {
		t.Fatal(err)
	}

	// check that PtData round-tripped correctly
	if form2.PtData == nil {
		t.Error("PtData was lost during round-trip")
		return
	}

	// use cmp to compare the PtData structures
	if diff := cmp.Diff(form1.PtData, form2.PtData, cmp.AllowUnexported(measure.PtData{})); diff != "" {
		t.Errorf("PtData round trip failed (-got +want):\n%s", diff)
	}
}

// TestFormWithStructParent verifies that StructParent fields are properly handled.
func TestFormWithStructParent(t *testing.T) {
	writer1, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm1 := pdf.NewResourceManager(writer1)

	form0 := &Form{
		Draw: func(w *graphics.Writer) error {
			w.SetFillColor(color.DeviceGray(0.5))
			w.Rectangle(0, 0, 100, 100)
			w.Fill()
			return nil
		},
		BBox:         pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		StructParent: structure.NewKey(42),
	}
	ref, _, err := pdf.ResourceManagerEmbed(rm1, form0)
	if err != nil {
		t.Fatal(err)
	}
	err = rm1.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = writer1.Close()
	if err != nil {
		t.Fatal(err)
	}

	x1 := pdf.NewExtractor(writer1)
	form1, err := Extract(x1, ref)
	if err != nil {
		t.Fatal(err)
	}

	// Verify StructParent was preserved
	if key, ok := form1.StructParent.Get(); !ok || key != 42 {
		t.Errorf("StructParent not preserved: got %v (present=%v), want 42", key, ok)
	}

	// Test with StructParent value 0 (edge case)
	writer2, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm2 := pdf.NewResourceManager(writer2)

	form0Zero := &Form{
		Draw: func(w *graphics.Writer) error {
			w.SetFillColor(color.DeviceGray(0.5))
			w.Rectangle(0, 0, 100, 100)
			w.Fill()
			return nil
		},
		BBox:         pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		StructParent: structure.NewKey(0),
	}
	ref2, _, err := pdf.ResourceManagerEmbed(rm2, form0Zero)
	if err != nil {
		t.Fatal(err)
	}
	err = rm2.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = writer2.Close()
	if err != nil {
		t.Fatal(err)
	}

	x2 := pdf.NewExtractor(writer2)
	form2, err := Extract(x2, ref2)
	if err != nil {
		t.Fatal(err)
	}

	// Verify StructParent value 0 was preserved
	if key, ok := form2.StructParent.Get(); !ok || key != 0 {
		t.Errorf("StructParent value 0 not preserved: got %v (present=%v), want 0", key, ok)
	}
}
