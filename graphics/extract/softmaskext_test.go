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
	"slices"
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/group"
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

	// Embedding a copy with no provenance entry of its own forces the
	// mask's Embed method to run.  G keeps its provenance and is not
	// re-embedded, while TR keeps whatever provenance the decoder gave it.
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

// TestSoftMaskBC checks that the reader drops a backdrop color which is
// malformed, does not match the group color space, or belongs to an Alpha
// mask, and keeps the rest of the mask.
func TestSoftMaskBC(t *testing.T) {
	for _, tc := range []struct {
		name   string
		s      pdf.Name
		cs     color.Space
		bc     pdf.Object
		wantBC []float64
	}{
		{"valid", "Luminosity", color.SpaceDeviceGray, pdf.Array{pdf.Number(0.5)}, []float64{0.5}},
		{"wrong length", "Luminosity", color.SpaceDeviceGray, pdf.Array{pdf.Number(0.5), pdf.Number(0.5)}, nil},
		{"non-numeric", "Luminosity", color.SpaceDeviceGray, pdf.Array{pdf.Name("X")}, nil},
		{"not an array", "Luminosity", color.SpaceDeviceGray, pdf.Integer(1), nil},
		{"no color space", "Luminosity", nil, pdf.Array{pdf.Number(0.5)}, nil},
		{"alpha mask", "Alpha", color.SpaceDeviceGray, pdf.Array{pdf.Number(0.5)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer, _ := memfile.NewPDFWriter(t, pdf.V1_4, nil)
			rm := pdf.NewResourceManager(writer)
			gRef, err := rm.Embed(transparencyGroup(tc.cs))
			if err != nil {
				t.Fatal(err)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}
			dict := pdf.Dict{
				"S":  tc.s,
				"G":  gRef,
				"BC": tc.bc,
			}

			x := pdf.NewExtractor(writer)
			got, err := extract.SoftMaskDict(pdf.CursorAt(x, nil), dict, false)
			if err != nil {
				t.Fatal(err)
			}
			m := got.(*softclip.Mask)
			if !slices.Equal(m.BC, tc.wantBC) {
				t.Errorf("BC = %v, want %v", m.BC, tc.wantBC)
			}
		})
	}
}

func transparencyGroup(cs color.Space) *form.Form {
	return &form.Form{
		BBox:   pdf.Rectangle{URx: 100, URy: 100},
		Matrix: matrix.Identity,
		Res:    &content.Resources{},
		Group:  &group.TransparencyAttributes{CS: cs, SingleUse: true},
	}
}
