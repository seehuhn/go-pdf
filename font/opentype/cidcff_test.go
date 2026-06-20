// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package opentype_test

import (
	"math"
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestCompositeCFFRoundTrip embeds an OpenType+CFF composite font and reads
// it back from the resulting PDF, checking the identifying fields.
func TestCompositeCFFRoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			fontData := makefont.OpenType()
			d := embedCompositeCFF(t, v, fontData)

			if d.PostScriptName != fontData.PostScriptName() {
				t.Errorf("PostScriptName: got %q, want %q",
					d.PostScriptName, fontData.PostScriptName())
			}
			if len(d.SubsetTag) != 6 {
				t.Errorf("SubsetTag: got %q, want 6 uppercase letters", d.SubsetTag)
			}
		})
	}
}

// TestCompositeCFFDescriptor verifies that the descriptor of a composite
// OpenType+CFF font reports metrics in PDF glyph space (1/1000 em),
// follows the Ascent-Descent+LineGap convention for Leading, and computes
// StemV using FD 0's effective font matrix (top-level matrix composed
// with the per-FD matrix).  This last point is what makes the CID-keyed
// path different from the simple variants: a bug that scaled StemV by
// the top-level matrix alone would produce the wrong value here.
func TestCompositeCFFDescriptor(t *testing.T) {
	fontData := makefont.OpenTypeCID2()
	outlines := fontData.Outlines.(*cff.Outlines)

	// Inject a non-identity per-FD matrix on FD 0 plus a known StdVW.  The
	// non-identity matrix is what differentiates "use FD 0's effective
	// matrix" from "use top-level matrix only" in the StemV computation.
	outlines.FontMatrices[0] = matrix.Matrix{0.5, 0, 0, 0.5, 0, 0}
	const stdVW = 100.0
	outlines.Private[0].StdVW = stdVW

	fd := embedCompositeCFF(t, pdf.V2_0, fontData).Descriptor
	checkDescriptorMetrics(t, fd, fontData)

	// Descriptor doc declares "StemV: 0 = unknown"; we have data, so the
	// descriptor must surface a non-zero value.
	if fd.StemV == 0 {
		t.Errorf("StemV: got 0, want non-zero (source StdVW = %v)", stdVW)
	}

	// StemV must scale by FD 0's effective matrix (per-FD * top-level),
	// not just the top-level matrix.
	fd0 := outlines.FDMatrix(0, fontData.AsCFF().FontMatrix)
	if want := math.Round(stdVW * fd0[0] * 1000); fd.StemV != want {
		t.Errorf("StemV: got %v, want %v (FD 0 effective matrix = %v)",
			fd.StemV, want, fd0)
	}
}

func embedCompositeCFF(t *testing.T, v pdf.Version, fontData *sfnt.Font) *dict.CIDFontType0 {
	t.Helper()
	w, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(w)

	F, err := opentype.NewComposite(fontData, nil)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	gg := F.Layout(nil, 12, "Hello")
	for _, g := range gg.Seq {
		_, _ = F.Encode(g.GID, g.Text)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := dictObj.(*dict.CIDFontType0)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}
	return d
}
