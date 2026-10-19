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

package image

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestJPXOldHeader checks that JPXDecode images, stencil masks and soft
// masks are read from files whose header predates JPXDecode.
func TestJPXOldHeader(t *testing.T) {
	src := jpxTestImage(6, 4, []int{1}, 60)
	data := encodeJPX(t, src)
	entries := pdf.Dict{
		"Width":     pdf.Integer(6),
		"Height":    pdf.Integer(4),
		"ImageMask": pdf.Boolean(true),
		"Filter":    pdf.Name("JPXDecode"),
	}
	c, ref := writeImageStream(t, pdf.V1_4, entries, data)
	m, err := ExtractMask(c, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	want := flateMaskAlpha(t, src.Planes[0].Pix, 6, 4, false)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mask alpha differs (-want +got):\n%s", diff)
	}

	src = jpxTestImage(6, 4, []int{8}, 61)
	data = encodeJPX(t, src)
	entries = pdf.Dict{
		"Width":      pdf.Integer(6),
		"Height":     pdf.Integer(4),
		"ColorSpace": pdf.Name("DeviceGray"),
		"Filter":     pdf.Name("JPXDecode"),
	}
	c, ref = writeImageStream(t, pdf.V1_4, entries, data)
	sm, err := ExtractSoftMask(c, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	got, err = sm.LoadAlpha()
	if err != nil {
		t.Fatal(err)
	}
	want = flateSoftMaskAlpha(t, src.Planes[0].Pix, 6, 4, 8, nil)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("soft mask alpha differs (-want +got):\n%s", diff)
	}

	d, err := ExtractDict(c, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Load(); err != nil {
		t.Error(err)
	}
}

// TestJPXFilterChain checks that the filters preceding JPXDecode are
// applied, independently of the file version.
func TestJPXFilterChain(t *testing.T) {
	src := jpxTestImage(6, 4, []int{1}, 62)

	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	zw.Write(encodeJPX(t, src))
	zw.Close()
	body := []byte(hex.EncodeToString(buf.Bytes()) + ">")

	filters := pdf.Array{pdf.Name("ASCIIHexDecode"), pdf.Name("FlateDecode"), pdf.Name("JPXDecode")}
	for name, parms := range map[string]pdf.Object{
		"no parms":    nil,
		"short parms": pdf.Array{nil},
		"full parms":  pdf.Array{nil, pdf.Dict{}, nil},
	} {
		entries := pdf.Dict{
			"Width":     pdf.Integer(6),
			"Height":    pdf.Integer(4),
			"ImageMask": pdf.Boolean(true),
			"Filter":    filters,
		}
		if parms != nil {
			entries["DecodeParms"] = parms
		}
		c, ref := writeImageStream(t, pdf.V1_1, entries, body)
		m, err := ExtractMask(c, ref, false)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got, err := m.LoadAlpha()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := flateMaskAlpha(t, src.Planes[0].Pix, 6, 4, false)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%s: alpha differs (-want +got):\n%s", name, diff)
		}
	}
}

// reembedDict embeds obj in a new file and returns the written stream
// dictionary.
func reembedDict(t *testing.T, obj pdf.Embedder) pdf.Dict {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(obj)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	stm, err := pdf.CursorAt(pdf.NewExtractor(w), nil).Stream(ref)
	if err != nil {
		t.Fatal(err)
	}
	return stm.Dict
}

// TestWriteStreamDropsSourceEntries checks that entries the reader drops
// or ignores are not copied into the written stream.
func TestWriteStreamDropsSourceEntries(t *testing.T) {
	c, ref := writeImageStream(t, pdf.V2_0, pdf.Dict{
		"Width":            pdf.Integer(2),
		"Height":           pdf.Integer(2),
		"ColorSpace":       pdf.Name("DeviceGray"),
		"BitsPerComponent": pdf.Integer(8),
		"Mask":             pdf.Array{pdf.Integer(5), pdf.Integer(300)},
		"SMaskInData":      pdf.Integer(1),
		"Private":          pdf.Name("X"),
	}, []byte{1, 2, 3, 4})
	d, err := ExtractDict(c, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	dict := reembedDict(t, d)
	for _, key := range []pdf.Name{"Mask", "SMaskInData", "Private"} {
		if _, ok := dict[key]; ok {
			t.Errorf("image: %s copied from source", key)
		}
	}

	sm, err := ExtractSoftMask(c, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	dict = reembedDict(t, sm)
	for _, key := range []pdf.Name{"Mask", "SMaskInData", "Private"} {
		if _, ok := dict[key]; ok {
			t.Errorf("soft mask: %s copied from source", key)
		}
	}
}
