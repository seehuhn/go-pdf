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

package pdf_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// writeSOS embeds s into a fresh in-memory PDF, stores the resulting object
// under /V of a container dictionary, and returns the closed writer (usable as
// a Getter) together with the container reference.
func writeSOS(t *testing.T, s pdf.StringOrStream) (*pdf.Writer, pdf.Reference) {
	t.Helper()
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	ref := w.Alloc()
	obj, err := rm.Embed(s)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if err := w.Put(ref, pdf.Dict{"V": obj}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return w, ref
}

// readSOS reads back the /V value stored under the container reference.
func readSOS(t *testing.T, r pdf.Getter, ref pdf.Reference) pdf.StringOrStream {
	t.Helper()
	c := pdf.NewCursor(r)
	d, err := c.Dict(ref)
	if err != nil {
		t.Fatalf("Dict: %v", err)
	}
	got, err := c.StringOrStream(d["V"])
	if err != nil {
		t.Fatalf("StringOrStream: %v", err)
	}
	return got
}

func TestStringOrStreamRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   pdf.StringOrStream
	}{
		{"inline ascii", pdf.StringOrStream{Value: "Hello, world"}},
		{"inline empty", pdf.StringOrStream{Value: ""}},
		{"inline unicode", pdf.StringOrStream{Value: "café — résumé — 日本語"}},
		{"stream ascii", pdf.StringOrStream{Value: "var x = 1;\n", IsStream: true}},
		{"stream empty", pdf.StringOrStream{Value: "", IsStream: true}},
		{"stream unicode", pdf.StringOrStream{Value: "<body>café 日本語</body>", IsStream: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, ref := writeSOS(t, tc.in)
			got := readSOS(t, w, ref)
			if diff := cmp.Diff(tc.in, got); diff != "" {
				t.Errorf("round trip failed (-want +got):\n%s", diff)
			}
		})
	}
}

// streamBytes returns the unencoded bytes of the stream stored under /V of the
// container reference.
func streamBytes(t *testing.T, r pdf.Getter, ref pdf.Reference) []byte {
	t.Helper()
	c := pdf.NewCursor(r)
	d, err := c.Dict(ref)
	if err != nil {
		t.Fatalf("Dict: %v", err)
	}
	strm, err := c.Stream(d["V"])
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	data, err := pdf.ReadAll(r, nil, strm, 1<<20)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return data
}

// TestStringOrStreamEmbedStreamIsTextStream checks that the stream form is
// written as a text stream: a value that cannot be PDFDocEncoded is stored with
// a UTF-16BE byte order marker, not as raw UTF-8.
func TestStringOrStreamEmbedStreamIsTextStream(t *testing.T) {
	in := pdf.StringOrStream{Value: "日本語", IsStream: true}
	w, ref := writeSOS(t, in)

	data := streamBytes(t, w, ref)
	if !bytes.HasPrefix(data, []byte{0xFE, 0xFF}) {
		t.Errorf("stream form lacks UTF-16BE BOM: got % x", data)
	}
	if bytes.Equal(data, []byte(in.Value)) {
		t.Errorf("stream form wrote raw UTF-8 bytes instead of a text stream")
	}

	if got := readSOS(t, w, ref); got != in {
		t.Errorf("round trip failed: got %+v, want %+v", got, in)
	}
}

// TestStringOrStreamEmbedStreamPDFDocEncoded checks that a value within
// PDFDocEncoding is stored compactly, without a spurious byte order marker.
func TestStringOrStreamEmbedStreamPDFDocEncoded(t *testing.T) {
	in := pdf.StringOrStream{Value: "café", IsStream: true}
	w, ref := writeSOS(t, in)

	data := streamBytes(t, w, ref)
	if bytes.HasPrefix(data, []byte{0xFE, 0xFF}) || bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		t.Errorf("PDFDocEncodable value gained a byte order marker: got % x", data)
	}

	if got := readSOS(t, w, ref); got != in {
		t.Errorf("round trip failed: got %+v, want %+v", got, in)
	}
}

// TestStringOrStreamReadForeignTextStream checks that a conforming text stream
// produced elsewhere (UTF-16BE with a byte order marker) decodes to its logical
// text rather than to its raw bytes.
func TestStringOrStreamReadForeignTextStream(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	strmRef := w.Alloc()
	body, err := w.OpenStream(strmRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	// UTF-16BE BOM followed by U+00E9 ("é")
	if _, err := body.Write([]byte{0xFE, 0xFF, 0x00, 0xE9}); err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{"V": strmRef}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	got := readSOS(t, w, ref)
	want := pdf.StringOrStream{Value: "é", IsStream: true}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected value (-want +got):\n%s", diff)
	}
}

// TestStringOrStreamFormsInterchangeable checks that the inline and stream forms
// of the same logical text decode to the same Value, differing only in IsStream.
func TestStringOrStreamFormsInterchangeable(t *testing.T) {
	const text = "café — 日本語"

	wInline, refInline := writeSOS(t, pdf.StringOrStream{Value: text})
	wStream, refStream := writeSOS(t, pdf.StringOrStream{Value: text, IsStream: true})

	inline := readSOS(t, wInline, refInline)
	stream := readSOS(t, wStream, refStream)

	if inline.Value != stream.Value {
		t.Errorf("forms disagree on Value: inline %q, stream %q", inline.Value, stream.Value)
	}
	if inline.IsStream || !stream.IsStream {
		t.Errorf("unexpected forms: inline.IsStream=%v, stream.IsStream=%v", inline.IsStream, stream.IsStream)
	}
}

// TestStringOrStreamEmbedInlineStaysDirect checks that the inline form is
// embedded as a direct string object, not promoted to an indirect reference.
func TestStringOrStreamEmbedInlineStaysDirect(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	obj, err := rm.Embed(pdf.StringOrStream{Value: "inline"})
	if err != nil {
		t.Fatal(err)
	}
	if _, isRef := obj.(pdf.Reference); isRef {
		t.Errorf("inline value was promoted to an indirect reference")
	}
	if _, ok := obj.(pdf.String); !ok {
		t.Errorf("expected a string object, got %T", obj)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestStringOrStreamEmbedStreamRequiresV15 checks that the stream form, a text
// stream introduced in PDF 1.5, is rejected when writing an earlier version
// while the inline form remains allowed.
func TestStringOrStreamEmbedStreamRequiresV15(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_4, nil)
	rm := pdf.NewResourceManager(w)

	if _, err := rm.Embed(pdf.StringOrStream{Value: "x", IsStream: true}); err == nil {
		t.Error("stream form accepted in PDF 1.4")
	}
	if _, err := rm.Embed(pdf.StringOrStream{Value: "x"}); err != nil {
		t.Errorf("inline form rejected in PDF 1.4: %v", err)
	}
}

// TestStringOrStreamEmbedDeduplicatesStreams checks that two identical
// stream-form values share a single stream object, while distinct values do
// not.
func TestStringOrStreamEmbedDeduplicatesStreams(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	s := pdf.StringOrStream{Value: "shared script", IsStream: true}
	a, err := rm.Embed(s)
	if err != nil {
		t.Fatal(err)
	}
	b, err := rm.Embed(s)
	if err != nil {
		t.Fatal(err)
	}
	refA, okA := a.(pdf.Reference)
	refB, okB := b.(pdf.Reference)
	if !okA || !okB {
		t.Fatalf("stream form did not embed as references: %T, %T", a, b)
	}
	if refA != refB {
		t.Errorf("identical streams not deduplicated: %v vs %v", refA, refB)
	}

	other, err := rm.Embed(pdf.StringOrStream{Value: "different script", IsStream: true})
	if err != nil {
		t.Fatal(err)
	}
	if other == a {
		t.Errorf("distinct streams collapsed to the same object")
	}

	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestStringOrStreamReadBareString checks that a value stored as a plain string
// (not produced by Embed) reads back as the inline form.
func TestStringOrStreamReadBareString(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{"V": pdf.String("plain text")}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	got := readSOS(t, w, ref)
	want := pdf.StringOrStream{Value: "plain text"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected value (-want +got):\n%s", diff)
	}
}

// TestStringOrStreamReadReference checks that a reference to a string resolves.
func TestStringOrStreamReadReference(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	strRef := w.Alloc()
	if err := w.Put(strRef, pdf.TextString("via reference")); err != nil {
		t.Fatal(err)
	}
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{"V": strRef}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	got := readSOS(t, w, ref)
	want := pdf.StringOrStream{Value: "via reference"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected value (-want +got):\n%s", diff)
	}
}

// TestStringOrStreamReadAbsent checks that a nil object yields the zero value.
func TestStringOrStreamReadAbsent(t *testing.T) {
	got, err := pdf.NewCursor(memfileGetter(t)).StringOrStream(nil)
	if err != nil {
		t.Fatalf("StringOrStream: %v", err)
	}
	if diff := cmp.Diff(pdf.StringOrStream{}, got); diff != "" {
		t.Errorf("unexpected value (-want +got):\n%s", diff)
	}
}

// TestStringOrStreamReadWrongType checks that a value of the wrong type is
// treated as absent rather than producing an error (read-permissive).
func TestStringOrStreamReadWrongType(t *testing.T) {
	got, err := pdf.NewCursor(memfileGetter(t)).StringOrStream(pdf.Integer(42))
	if err != nil {
		t.Fatalf("StringOrStream: %v", err)
	}
	if diff := cmp.Diff(pdf.StringOrStream{}, got); diff != "" {
		t.Errorf("unexpected value (-want +got):\n%s", diff)
	}
}

func memfileGetter(t *testing.T) pdf.Getter {
	t.Helper()
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return w
}
