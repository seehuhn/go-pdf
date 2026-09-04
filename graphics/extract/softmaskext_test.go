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

package extract_test

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/softclip"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestSoftMaskTRProvenance regresses a bug where a soft mask's indirect
// /TR transfer function lost its provenance on decode, so re-embedding the
// mask wrote a second copy of the function instead of reusing the original
// object.
func TestSoftMaskTRProvenance(t *testing.T) {
	w0, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	funcRef := w0.Alloc()
	if err := w0.Put(funcRef, pdf.Dict{
		"FunctionType": pdf.Integer(2),
		"Domain":       pdf.Array{pdf.Number(0), pdf.Number(1)},
		"N":            pdf.Number(1),
	}); err != nil {
		t.Fatal(err)
	}

	formRef := w0.Alloc()
	formBody, err := w0.OpenStream(formRef, pdf.Dict{
		"Type":    pdf.Name("XObject"),
		"Subtype": pdf.Name("Form"),
		"BBox":    pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(1), pdf.Integer(1)},
		"Group": pdf.Dict{
			"Type": pdf.Name("Group"),
			"S":    pdf.Name("Transparency"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := formBody.Close(); err != nil {
		t.Fatal(err)
	}

	smRef := w0.Alloc()
	if err := w0.Put(smRef, pdf.Dict{
		"S":  pdf.Name("Luminosity"),
		"G":  formRef,
		"TR": funcRef,
	}); err != nil {
		t.Fatal(err)
	}
	if err := w0.Close(); err != nil {
		t.Fatal(err)
	}

	orig := bytes.Clone(f.Data)
	w, err := pdf.NewUpdater(f, int64(len(orig)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	sc, err := pdf.Decode(c, smRef, extract.SoftMaskDict)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := sc.(*softclip.Mask)
	if !ok {
		t.Fatalf("decoded %T, want *softclip.Mask", sc)
	}
	if got := w.Origin(m.TR); got == 0 {
		t.Fatal("decoded TR function has no provenance")
	}

	// force the mask's Embed method to actually run, by embedding a copy
	// with no provenance entry of its own; G keeps its provenance so it is
	// not re-embedded, while TR keeps whatever provenance the fix under
	// test does or does not give it
	cp := *m
	rm := pdf.NewResourceManager(w)
	if _, err := rm.Embed(&cp); err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	if n := bytes.Count(f.Data[len(orig):], []byte("/FunctionType 2")); n != 0 {
		t.Errorf("function written again: found %d copies in the update", n)
	}
}
