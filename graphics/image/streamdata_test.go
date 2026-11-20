// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package image

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"errors"
	"io"
	"testing"
	"testing/iotest"

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

// TestJPXStreamAbbreviations verifies that jpxStream keeps the filter
// parameters given under the abbreviated key /DP.
func TestJPXStreamAbbreviations(t *testing.T) {
	parms := pdf.Dict{"Predictor": pdf.Integer(12)}
	stream := pdf.NewStream(pdf.Dict{
		"Filter": pdf.Array{pdf.Name("Fl"), pdf.Name("JPXDecode")},
		"DP":     pdf.Array{parms, nil},
	}, nil)
	res, err := jpxStream(nil, stream, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := pdf.Dict{
		"Filter":      pdf.Array{pdf.Name("Fl")},
		"DecodeParms": pdf.Array{parms},
	}
	if diff := cmp.Diff(want, res.Dict); diff != "" {
		t.Errorf("wrong stream dict (-want +got):\n%s", diff)
	}
}

// truncatedFlate returns the first half of the Flate-compressed data.
func truncatedFlate(data []byte) []byte {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	zw.Write(data)
	zw.Close()
	return buf.Bytes()[:buf.Len()/2]
}

// patternBytes returns n bytes which do not compress well.
func patternBytes(n int) []byte {
	data := make([]byte, n)
	for i := range data {
		data[i] = byte(i*i*31 + i*7)
	}
	return data
}

// TestTruncatedImageStream checks that an image whose compressed stream is
// cut short can be read, and that a read-write-read cycle is stable.
func TestTruncatedImageStream(t *testing.T) {
	const width, height = 32, 32
	c, ref := writeImageStream(t, pdf.V1_7, pdf.Dict{
		"Width":            pdf.Integer(width),
		"Height":           pdf.Integer(height),
		"ColorSpace":       pdf.Name("DeviceGray"),
		"BitsPerComponent": pdf.Integer(8),
		"Filter":           pdf.Name("FlateDecode"),
	}, truncatedFlate(patternBytes(width*height)))
	d1, err := ExtractDict(c, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	data1, err := d1.Load()
	if err != nil {
		t.Fatal(err)
	}

	r, err := d1.Rows()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for range r.All() {
		n++
	}
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
	if n != height {
		t.Errorf("got %d rows, want %d", n, height)
	}

	d2, err := roundTrip(t, pdf.V1_7, d1, ExtractDict)
	if err != nil {
		t.Fatal(err)
	}
	data2, err := d2.Load()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(data1.Pix, data2.Pix); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// TestTruncatedMaskStream checks that stencil masks and soft masks whose
// compressed stream is cut short can be read, and that a read-write-read
// cycle is stable.
func TestTruncatedMaskStream(t *testing.T) {
	const width, height = 64, 32

	t.Run("Mask", func(t *testing.T) {
		c, ref := writeImageStream(t, pdf.V1_7, pdf.Dict{
			"Width":     pdf.Integer(width),
			"Height":    pdf.Integer(height),
			"ImageMask": pdf.Boolean(true),
			"Filter":    pdf.Name("FlateDecode"),
		}, truncatedFlate(patternBytes(width/8*height)))
		m1, err := ExtractMask(c, ref, false)
		if err != nil {
			t.Fatal(err)
		}
		a1, err := m1.LoadAlpha()
		if err != nil {
			t.Fatal(err)
		}
		m2, err := roundTrip(t, pdf.V1_7, m1, ExtractMask)
		if err != nil {
			t.Fatal(err)
		}
		a2, err := m2.LoadAlpha()
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(a1, a2); diff != "" {
			t.Errorf("round trip failed (-want +got):\n%s", diff)
		}
	})

	t.Run("SoftMask", func(t *testing.T) {
		c, ref := writeImageStream(t, pdf.V1_7, pdf.Dict{
			"Width":            pdf.Integer(width),
			"Height":           pdf.Integer(height),
			"ColorSpace":       pdf.Name("DeviceGray"),
			"BitsPerComponent": pdf.Integer(8),
			"Filter":           pdf.Name("FlateDecode"),
		}, truncatedFlate(patternBytes(width*height)))
		sm1, err := ExtractSoftMask(c, ref, false)
		if err != nil {
			t.Fatal(err)
		}
		a1, err := sm1.LoadAlpha()
		if err != nil {
			t.Fatal(err)
		}
		sm2, err := roundTrip(t, pdf.V1_7, sm1, ExtractSoftMask)
		if err != nil {
			t.Fatal(err)
		}
		a2, err := sm2.LoadAlpha()
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(a1, a2); diff != "" {
			t.Errorf("round trip failed (-want +got):\n%s", diff)
		}
	})
}

// TestTruncatedJPXFilterChain checks that a JPXDecode image whose outer
// Flate stream is cut short, so that the JPEG 2000 decoder sees a partial
// codestream, can be read, and that a read-write-read cycle is stable.
func TestTruncatedJPXFilterChain(t *testing.T) {
	src := jpxTestImage(32, 32, []int{8, 8, 8}, 7)
	c, ref := writeImageStream(t, pdf.V2_0, pdf.Dict{
		"Width":  pdf.Integer(32),
		"Height": pdf.Integer(32),
		"Filter": pdf.Array{pdf.Name("FlateDecode"), pdf.Name("JPXDecode")},
	}, truncatedFlate(encodeJPX(t, src)))
	d1, err := ExtractDict(c, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	data1, err := d1.Load()
	if err != nil {
		t.Fatal(err)
	}
	d2, err := roundTrip(t, pdf.V2_0, d1, ExtractDict)
	if err != nil {
		t.Fatal(err)
	}
	data2, err := d2.Load()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(data1.Pix, data2.Pix); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// TestLenientReaderPassesSourceErrors checks that an error which does not
// report malformed content ends the read with that error.
func TestLenientReaderPassesSourceErrors(t *testing.T) {
	ioErr := errors.New("disk on fire")
	r := &lenientReader{r: iotest.ErrReader(ioErr), Closer: io.NopCloser(nil)}
	if _, err := io.ReadAll(r); err != ioErr {
		t.Errorf("got error %v, want %v", err, ioErr)
	}
}
