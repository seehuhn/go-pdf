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

package cidfont

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestType1DictRoundtrip(t *testing.T) {
	t.Skip()

	data, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(data)

	F1 := makefont.OpenTypeCID()
	fontCFF := F1.Outlines.(*cff.Outlines)

	q := F1.FontMatrix[3] * 1000
	fd := &font.Descriptor{
		FontName:     F1.PostScriptName(),
		FontFamily:   F1.FamilyName,
		IsFixedPitch: F1.IsFixedPitch(),
		IsSymbolic:   true,
		FontBBox:     F1.FontBBoxPDF().Rounded(),
		Ascent:       F1.Ascent.AsFloat(q),
		Descent:      F1.Descent.AsFloat(q),
		CapHeight:    F1.CapHeight.AsFloat(q),
		XHeight:      F1.XHeight.AsFloat(q),
		StemV:        fontCFF.Private[0].StdVW * (F1.FontMatrix[0] * 1000),
		StemH:        fontCFF.Private[0].StdHW * (F1.FontMatrix[3] * 1000),
	}
	ros := &cmap.CIDSystemInfo{Registry: "seehuhn.de", Ordering: "test"}
	dicts1 := &Type0Dict{
		Ref:            data.Alloc(),
		PostScriptName: F1.PostScriptName(),
		Descriptor:     fd,
		ROS:            ros,
		Encoding: &cmap.File{
			Name:  "Test",
			ROS:   ros,
			WMode: cmap.Horizontal,
			CodeSpaceRange: []charcode.Range{
				{Low: []byte{0x00}, High: []byte{0xFF}},
			},
		},
		Width:        map[cmap.CID]float64{}, // TODO(voss)
		DefaultWidth: F1.GlyphWidthPDF(0),
		Text: &cmap.ToUnicodeFile{
			CodeSpaceRange: []charcode.Range{
				{Low: []byte{0x00}, High: []byte{0xFF}},
			},
			Singles: []cmap.ToUnicodeSingle{},
		},
		GetFont: func() (Type0FontData, error) {
			return F1, nil
		},
	}

	lookup, err := F1.CMapTable.GetBest()
	if err != nil {
		t.Fatal(err)
	}
	for code, glyphName := range pdfenc.Standard.Encoding {
		rr := names.ToUnicode(glyphName, false)
		if len(rr) != 1 {
			continue
		}
		gid := lookup.Lookup(rr[0])
		if gid == 0 {
			continue
		}
		cid := cmap.CID(fontCFF.GIDToCID[gid])
		dicts1.Encoding.CIDSingles = append(dicts1.Encoding.CIDSingles,
			cmap.Single{Code: []byte{byte(code)}, Value: cid})
		dicts1.Width[cid] = F1.GlyphWidthPDF(gid)
		dicts1.Text.Singles = append(dicts1.Text.Singles,
			cmap.ToUnicodeSingle{Code: []byte{byte(code)}, Value: string(rr)})
	}

	err = dicts1.WriteToPDF(rm)
	if err != nil {
		t.Fatal(err)
	}

	dicts2, err := ExtractType0(data, dicts1.Ref)
	if err != nil {
		t.Fatal(err)
	}

	compareType0Dicts(t, dicts1, dicts2)

	F2Interface, err := dicts2.GetFont()
	if err != nil {
		t.Fatal(err)
	}
	F2, ok := F2Interface.(*sfnt.Font)
	if !ok {
		t.Fatalf("got %T, want *type1.Font", F2Interface)
	}
	compareOpentypeFont(t, F1, F2)
}

// compareType0Dicts compares two Type1Dicts.
// d1 must be the original, d2 the one that was read back from the PDF file.
func compareType0Dicts(t *testing.T, d1, d2 *Type0Dict) {
	t.Helper()

	if d1.Ref != d2.Ref {
		t.Errorf("Ref: got %s, want %s", d2.Ref, d1.Ref)
	}
	if d1.PostScriptName != d2.PostScriptName {
		t.Errorf("PostScriptName: got %s, want %s", d2.PostScriptName, d1.PostScriptName)
	}
	if d1.SubsetTag != d2.SubsetTag {
		t.Errorf("SubsetTag: got %s, want %s", d2.SubsetTag, d1.SubsetTag)
	}
	if d := cmp.Diff(d1.Descriptor, d2.Descriptor); d != "" {
		t.Errorf("Descriptor: %s", d)
	}

	if d := cmp.Diff(d1.Encoding, d2.Encoding); d != "" {
		t.Errorf("Encoding: %s", d)
	}

	if math.Abs(d1.DefaultWidth-d2.DefaultWidth) > 1e-6 {
		t.Errorf("DefaultWidth: got %f, want %f", d2.DefaultWidth, d1.DefaultWidth)
	}
	for cid, w1 := range d1.Width {
		w2, ok := d2.Width[cid]
		if !ok {
			w2 = d2.DefaultWidth
		}
		if math.Abs(w1-w2) > 1e-6 {
			t.Errorf("Width[%d]: got %f, want %f", cid, w2, w1)
		}
	}

	if d := cmp.Diff(d1.Text, d2.Text); d != "" {
		t.Errorf("Text: %s", d)
	}
}

// compareOpentypeFont compares two *cff.Font objects.
func compareOpentypeFont(t *testing.T, f1, f2 *sfnt.Font) {
	t.Helper()

	f1 = f1.Clone()
	o1 := f1.Outlines.(*cff.Outlines)
	f1.Outlines = nil
	f2 = f2.Clone()
	o2 := f2.Outlines.(*cff.Outlines)
	f2.Outlines = nil

	if d := cmp.Diff(f1, f2); d != "" {
		t.Errorf("FontInfo: %s", d)
	}
	_ = o1
	_ = o2
}
