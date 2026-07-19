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

package form_test

import (
	"io"
	"maps"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/opi"
	"seehuhn.de/go/pdf/graphics/reference"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/measure"
	"seehuhn.de/go/pdf/optional"
	"seehuhn.de/go/pdf/pieceinfo"
)

// testData implements pieceinfo.Data for testing
type testData struct {
	data string
}

func (t *testData) LastModified() time.Time {
	return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
}

func (t *testData) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	return pdf.String(t.data), nil
}

// makeTestForm creates a simple test form with a gray rectangle
func makeTestForm() *form.Form {
	b := builder.New(content.Form, nil, pdf.V2_0)
	b.SetFillColor(color.DeviceGray(0.5))
	b.Rectangle(0, 0, 100, 100)
	b.Fill()
	return &form.Form{
		Content: &content.Operators{Ops: b.Stream},
		Res:     b.Resources,
		BBox:    pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
	}
}

// TestRead verifies that a form XObject read from one PDF file can be written
// to another PDF file.
func TestRead(t *testing.T) {
	writer1, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm1 := pdf.NewResourceManager(writer1)

	form0 := makeTestForm()
	ref, err := rm1.Embed(form0)
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
	form1, err := extract.Form(pdf.CursorAt(x1, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}

	writer2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm2 := pdf.NewResourceManager(writer2)
	ref2, err := rm2.Embed(form1)
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
	form2, err := extract.Form(pdf.CursorAt(x2, nil), ref2, false)
	if err != nil {
		t.Fatal(err)
	}

	// check that form1 and form2 are the same
	if !form1.Equal(form2) {
		t.Errorf("round trip failed")
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

	form0 := makeTestForm()
	form0.PieceInfo = testPieceInfo
	form0.LastModified = time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	ref, err := rm1.Embed(form0)
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
	form1, err := extract.Form(pdf.CursorAt(x1, nil), ref, false)
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
	ref2, err := rm2.Embed(form1)
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
	form2, err := extract.Form(pdf.CursorAt(x2, nil), ref2, false)
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

	form := makeTestForm()
	form.PieceInfo = testPieceInfo
	// LastModified is intentionally not set to trigger validation error

	_, err := rm.Embed(form)
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

	form0 := makeTestForm()
	form0.PtData = testPtData

	ref, err := rm1.Embed(form0)
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
	form1, err := extract.Form(pdf.CursorAt(x1, nil), ref, false)
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
	ref2, err := rm2.Embed(form1)
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
	form2, err := extract.Form(pdf.CursorAt(x2, nil), ref2, false)
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

	form0 := makeTestForm()
	form0.StructParent = optional.NewUInt(42)
	ref, err := rm1.Embed(form0)
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
	form1, err := extract.Form(pdf.CursorAt(x1, nil), ref, false)
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

	form0Zero := makeTestForm()
	form0Zero.StructParent = optional.NewUInt(0)
	ref2, err := rm2.Embed(form0Zero)
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
	form2, err := extract.Form(pdf.CursorAt(x2, nil), ref2, false)
	if err != nil {
		t.Fatal(err)
	}

	// Verify StructParent value 0 was preserved
	if key, ok := form2.StructParent.Get(); !ok || key != 0 {
		t.Errorf("StructParent value 0 not preserved: got %v (present=%v), want 0", key, ok)
	}
}

// roundTripForm embeds f, extracts it back, and returns the extracted form.
func roundTripForm(t *testing.T, version pdf.Version, f *form.Form) *form.Form {
	t.Helper()
	writer, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(writer)
	ref, err := rm.Embed(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	x := pdf.NewExtractor(writer)
	got, err := extract.Form(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestFormWithRef verifies that a reference XObject's Ref entry survives a
// form XObject read/write cycle.
func TestFormWithRef(t *testing.T) {
	form0 := makeTestForm()
	form0.Ref = &reference.Dict{
		F:         &file.Specification{FileName: "target.pdf", AFRelationship: file.RelationshipUnspecified},
		PageIndex: 2,
	}

	form1 := roundTripForm(t, pdf.V2_0, form0)
	if form1.Ref == nil {
		t.Fatal("Ref was lost during extraction")
	}

	form2 := roundTripForm(t, pdf.V2_0, form1)
	if !form1.Equal(form2) {
		t.Error("Ref round trip failed")
	}
}

// TestFormWithOPI verifies that an OPI entry survives a form XObject read/write
// cycle. OPI is deprecated in PDF 2.0, so the test uses PDF 1.7.
func TestFormWithOPI(t *testing.T) {
	form0 := makeTestForm()
	form0.OPI = &opi.V20{
		F:         &file.Specification{FileName: "proxy.tif", AFRelationship: file.RelationshipUnspecified},
		MainImage: pdf.String("/vol/full.tif"),
		Overprint: true,
	}

	form1 := roundTripForm(t, pdf.V1_7, form0)
	if form1.OPI == nil {
		t.Fatal("OPI was lost during extraction")
	}

	form2 := roundTripForm(t, pdf.V1_7, form1)
	if !form1.Equal(form2) {
		t.Error("OPI round trip failed")
	}
}

// writeRawForm writes a form XObject dict directly, bypassing form.Embed, so
// tests can construct dicts without a /Resources entry.
func writeRawForm(t *testing.T, version pdf.Version, withResources bool) (*pdf.Writer, pdf.Reference) {
	t.Helper()
	writer, _ := memfile.NewPDFWriter(version, nil)
	ref := writer.Alloc()
	dict := pdf.Dict{
		"Subtype": pdf.Name("Form"),
		"BBox":    &pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
	}
	if withResources {
		dict["Resources"] = pdf.Dict{}
	}
	stm, err := writer.OpenStream(ref, dict)
	if err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return writer, ref
}

// TestExtractFormPre20MissingResources verifies that a pre-2.0 form XObject
// without a /Resources entry extracts with Res == nil, allowing the renderer
// to inherit from the page (PDF 2.0 Â§7.8.3 Note 3).
func TestExtractFormPre20MissingResources(t *testing.T) {
	writer, ref := writeRawForm(t, pdf.V1_7, false)
	x := pdf.NewExtractor(writer)
	f, err := extract.Form(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	if f.Res != nil {
		t.Errorf("expected nil Res for pre-2.0 form without /Resources, got %#v", f.Res)
	}
}

// TestExtractForm20MissingResources verifies that a 2.0 form XObject without a
// /Resources entry is normalised to an empty Resources, since the spec
// requires the entry.
func TestExtractForm20MissingResources(t *testing.T) {
	writer, ref := writeRawForm(t, pdf.V2_0, false)
	x := pdf.NewExtractor(writer)
	f, err := extract.Form(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	if f.Res == nil {
		t.Errorf("expected non-nil empty Res for 2.0 form without /Resources, got nil")
	}
}

// TestExtractFormEmptyResources verifies that an explicit empty /Resources
// entry extracts as a non-nil empty Resources (not nil).
func TestExtractFormEmptyResources(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			writer, ref := writeRawForm(t, v, true)
			x := pdf.NewExtractor(writer)
			f, err := extract.Form(pdf.CursorAt(x, nil), ref, false)
			if err != nil {
				t.Fatal(err)
			}
			if f.Res == nil {
				t.Errorf("expected non-nil Res for form with explicit empty /Resources, got nil")
			}
		})
	}
}

// TestEmbedNilResRejected20 verifies that writing a form with Res == nil at
// PDF 2.0 fails, because the spec requires a /Resources entry.
func TestEmbedNilResRejected20(t *testing.T) {
	writer, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(writer)
	f := &form.Form{
		BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
	}
	if _, err := rm.Embed(f); err == nil {
		t.Error("expected error embedding nil-Res form at PDF 2.0")
	}
	// best-effort cleanup; the failed Embed may leave the writer in a state
	// where Close also reports an error, which we ignore for this test
	_ = rm.Close()
	_ = writer.Close()
}

// TestEmbedNilResOmitsResources17 verifies that writing a form with Res == nil
// at PDF 1.7 is accepted, and the resulting stream dict has no /Resources
// entry.  Round-trip extraction yields Res == nil again.
func TestEmbedNilResOmitsResources17(t *testing.T) {
	writer, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(writer)
	f := &form.Form{
		BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
	}
	ref, err := rm.Embed(f)
	if err != nil {
		t.Fatalf("embedding nil-Res form at PDF 1.7: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	// verify the written dict has no /Resources entry
	stm, err := pdf.NewCursor(writer).Stream(ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stm.Dict["Resources"]; ok {
		t.Error("expected no /Resources entry for nil-Res form at PDF 1.7")
	}

	// round trip
	x := pdf.NewExtractor(writer)
	f2, err := extract.Form(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	if f2.Res != nil {
		t.Errorf("round trip: expected nil Res, got %#v", f2.Res)
	}
}

// TestFormWithAssociatedFiles verifies that AssociatedFiles (AF) are properly
// handled during form XObject read/write cycles.
func TestFormWithAssociatedFiles(t *testing.T) {
	writer1, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm1 := pdf.NewResourceManager(writer1)

	// create test associated files
	testSpec := &file.Specification{
		FileName:        "source.xml",
		FileNameUnicode: "source.xml",
		Description:     "Source XML data",
		AFRelationship:  file.RelationshipSource,
		EmbeddedFiles: map[string]*file.Stream{
			"UF": {
				MimeType: "application/xml",
				ModDate:  time.Date(2026, 1, 9, 12, 0, 0, 0, time.UTC),
				WriteData: func(w io.Writer) error {
					_, err := w.Write([]byte("<data>test content</data>"))
					return err
				},
			},
		},
	}

	form0 := makeTestForm()
	form0.AssociatedFiles = []*file.Specification{testSpec}

	ref, err := rm1.Embed(form0)
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
	form1, err := extract.Form(pdf.CursorAt(x1, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}

	// verify AssociatedFiles was preserved
	if form1.AssociatedFiles == nil {
		t.Error("AssociatedFiles was not preserved during extraction")
		return
	}
	if len(form1.AssociatedFiles) != 1 {
		t.Errorf("AssociatedFiles length mismatch: got %d, want 1", len(form1.AssociatedFiles))
		return
	}

	// check AssociatedFiles content
	extractedSpec := form1.AssociatedFiles[0]
	if extractedSpec.FileNameUnicode != "source.xml" {
		t.Errorf("AssociatedFiles FileNameUnicode mismatch: got %s, want source.xml", extractedSpec.FileNameUnicode)
	}
	if extractedSpec.AFRelationship != file.RelationshipSource {
		t.Errorf("AssociatedFiles AFRelationship mismatch: got %s, want %s", extractedSpec.AFRelationship, file.RelationshipSource)
	}

	// test round-trip
	writer2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm2 := pdf.NewResourceManager(writer2)
	ref2, err := rm2.Embed(form1)
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
	form2, err := extract.Form(pdf.CursorAt(x2, nil), ref2, false)
	if err != nil {
		t.Fatal(err)
	}

	// check that AssociatedFiles round-tripped correctly
	if form2.AssociatedFiles == nil {
		t.Error("AssociatedFiles was lost during round-trip")
		return
	}
	if len(form2.AssociatedFiles) != 1 {
		t.Errorf("AssociatedFiles length mismatch after round-trip: got %d, want 1", len(form2.AssociatedFiles))
		return
	}

	// verify content matches
	if form2.AssociatedFiles[0].FileNameUnicode != form1.AssociatedFiles[0].FileNameUnicode {
		t.Errorf("AssociatedFiles FileNameUnicode round trip failed: got %s, want %s",
			form2.AssociatedFiles[0].FileNameUnicode, form1.AssociatedFiles[0].FileNameUnicode)
	}
}

// writeFormDict writes a form XObject dict with the given extra entries,
// bypassing form.Embed so that tests can construct dicts which Embed would
// refuse to produce.
func writeFormDict(t *testing.T, version pdf.Version, extra pdf.Dict) (*pdf.Writer, pdf.Reference) {
	t.Helper()
	writer, _ := memfile.NewPDFWriter(version, nil)
	ref := writer.Alloc()
	dict := pdf.Dict{
		"Subtype":   pdf.Name("Form"),
		"BBox":      &pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		"Resources": pdf.Dict{},
	}
	maps.Copy(dict, extra)
	stm, err := writer.OpenStream(ref, dict)
	if err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return writer, ref
}

// TestExtractKeepsBothVariants checks that a form which carries both printer's
// mark and trap network entries keeps both.  Neither set of entries makes the
// dictionary invalid; which of them applies depends on where the form is
// referenced from, which is not known here.
func TestExtractKeepsBothVariants(t *testing.T) {
	writer, ref := writeFormDict(t, pdf.V1_7, pdf.Dict{
		"MarkStyle": pdf.TextString("Colour bar"),
		"PCM":       pdf.Name("DeviceCMYK"),
	})

	x := pdf.NewExtractor(writer)
	f, err := extract.Form(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}

	if f.PrinterMark == nil {
		t.Error("expected the printer's mark entries to be kept")
	}
	if f.TrapNet == nil {
		t.Error("expected the trap network entries to be kept")
	}

	mustEmbed(t, pdf.V1_7, f)
}

// TestExtractKeepsTrapNetAt13 checks that a form carrying both sets of entries
// keeps the trap network at PDF 1.3, where printer's marks do not yet exist.
// Resolving a conflict between the two used to run before the version
// stripping, so that both sets were lost here.
func TestExtractKeepsTrapNetAt13(t *testing.T) {
	writer, ref := writeFormDict(t, pdf.V1_3, pdf.Dict{
		"MarkStyle": pdf.TextString("Colour bar"),
		"PCM":       pdf.Name("DeviceCMYK"),
	})

	x := pdf.NewExtractor(writer)
	f, err := extract.Form(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}

	if f.PrinterMark != nil {
		t.Error("expected the printer's mark entries to be dropped at PDF 1.3")
	}
	if f.TrapNet == nil {
		t.Error("expected the trap network entries to be kept")
	}

	mustEmbed(t, pdf.V1_3, f)
}

// TestExtractStripsFutureEntries checks that entries which the file's PDF
// version cannot carry are dropped on read, so that everything which can be
// read can also be written back.
func TestExtractStripsFutureEntries(t *testing.T) {
	for _, tc := range []struct {
		name string
		// tooOld is a version which cannot carry the entry, ok is one which can
		tooOld  pdf.Version
		ok      pdf.Version
		extra   pdf.Dict
		present func(*form.Form) bool
	}{
		{
			name:    "TrapNet before 1.3",
			tooOld:  pdf.V1_2,
			ok:      pdf.V1_3,
			extra:   pdf.Dict{"PCM": pdf.Name("DeviceCMYK")},
			present: func(f *form.Form) bool { return f.TrapNet != nil },
		},
		{
			name:    "PrinterMark before 1.4",
			tooOld:  pdf.V1_3,
			ok:      pdf.V1_4,
			extra:   pdf.Dict{"MarkStyle": pdf.TextString("Colour bar")},
			present: func(f *form.Form) bool { return f.PrinterMark != nil },
		},
		{
			name:   "StructParent before 1.3",
			tooOld: pdf.V1_2,
			ok:     pdf.V1_3,
			extra:  pdf.Dict{"StructParent": pdf.Integer(3)},
			present: func(f *form.Form) bool {
				_, ok := f.StructParent.Get()
				return ok
			},
		},
		{
			name:   "Group before 1.4",
			tooOld: pdf.V1_3,
			ok:     pdf.V1_4,
			extra: pdf.Dict{
				"Group": pdf.Dict{
					"Type": pdf.Name("Group"),
					"S":    pdf.Name("Transparency"),
				},
			},
			present: func(f *form.Form) bool { return f.Group != nil },
		},
		{
			name:   "Measure before 2.0",
			tooOld: pdf.V1_7,
			ok:     pdf.V2_0,
			extra: pdf.Dict{
				"Measure": pdf.Dict{
					"Type":    pdf.Name("Measure"),
					"Subtype": pdf.Name("RL"),
					"R":       pdf.TextString("1in = 1in"),
				},
			},
			present: func(f *form.Form) bool { return f.Measure != nil },
		},
		{
			name:   "OPI before 1.2",
			tooOld: pdf.V1_1,
			ok:     pdf.V1_2,
			extra: pdf.Dict{
				"OPI": pdf.Dict{
					"1.3": pdf.Dict{
						"Type":    pdf.Name("OPI"),
						"Version": pdf.Real(1.3),
						"F": pdf.Dict{
							"Type": pdf.Name("Filespec"),
							"F":    pdf.String("pic.tif"),
						},
						"Size": pdf.Array{pdf.Integer(100), pdf.Integer(100)},
						"CropRect": pdf.Array{pdf.Integer(0), pdf.Integer(100),
							pdf.Integer(100), pdf.Integer(0)},
						"Position": pdf.Array{
							pdf.Integer(0), pdf.Integer(0),
							pdf.Integer(0), pdf.Integer(100),
							pdf.Integer(100), pdf.Integer(100),
							pdf.Integer(100), pdf.Integer(0),
						},
					},
				},
			},
			present: func(f *form.Form) bool { return f.OPI != nil },
		},
		{
			name:   "Ref before 1.4",
			tooOld: pdf.V1_3,
			ok:     pdf.V1_4,
			extra: pdf.Dict{
				"Ref": pdf.Dict{
					"F": pdf.Dict{
						"Type": pdf.Name("Filespec"),
						"F":    pdf.String("target.pdf"),
					},
					"Page": pdf.Integer(0),
				},
			},
			present: func(f *form.Form) bool { return f.Ref != nil },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The entry must survive at a version which allows it, otherwise
			// the check below would pass for the wrong reason.
			writer, ref := writeFormDict(t, tc.ok, tc.extra)
			x := pdf.NewExtractor(writer)
			f, err := extract.Form(pdf.CursorAt(x, nil), ref, false)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.present(f) {
				t.Fatalf("entry does not survive extraction at %s", tc.ok)
			}

			writer, ref = writeFormDict(t, tc.tooOld, tc.extra)
			x = pdf.NewExtractor(writer)
			f, err = extract.Form(pdf.CursorAt(x, nil), ref, false)
			if err != nil {
				t.Fatal(err)
			}
			if tc.present(f) {
				t.Errorf("entry was not dropped at %s", tc.tooOld)
			}

			// the crucial part: what we read must be writable again
			mustEmbed(t, tc.tooOld, f)
		})
	}
}

// mustEmbed writes f to a new file of the given version and reads it back,
// checking that the value survives unchanged.  Writing alone only shows that
// the form is acceptable; the round trip shows that nothing was lost.
//
// Callers pass the version f was read at, and reading drops entries that
// version cannot carry, so a version error means the read side let something
// through.  Unlike the usual round-trip helper this one therefore fails on
// [pdf.IsWrongVersion] rather than skipping.
func mustEmbed(t *testing.T, version pdf.Version, f *form.Form) {
	t.Helper()

	out, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(out)
	obj, err := rm.Embed(f)
	if err != nil {
		t.Fatalf("cannot write the form back: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(out)
	f2, err := extract.Form(pdf.CursorAt(x, nil), obj, false)
	if err != nil {
		t.Fatalf("cannot read the form back: %v", err)
	}
	if diff := cmp.Diff(f, f2); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}
