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

package opaque

import (
	"bytes"
	"compress/zlib"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestStreamReader verifies that Reader returns the decoded body of
// the wrapped stream.
func TestStreamReader(t *testing.T) {
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	body := []byte("hello, stream world")
	ref := src.Alloc()
	w, err := src.OpenStream(ref, pdf.Dict{"Custom": pdf.Name("Demo")})
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	if _, err := w.Write(body); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close stream: %v", err)
	}
	src.GetMeta().Trailer["Quir:E"] = ref
	if err := src.Close(); err != nil {
		t.Fatalf("src.Close: %v", err)
	}

	srcX := pdf.NewExtractor(src)
	stream, err := pdf.CursorAt(srcX, nil).Stream(ref)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	s, err := ExtractStream(srcX, stream)
	if err != nil {
		t.Fatal(err)
	}

	rc, err := s.Reader()
	if err != nil {
		t.Fatalf("Reader: %v", err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("Reader: got %q, want %q", got, body)
	}
}

// TestStreamWriteAtCrossFile verifies that WriteAt keeps the
// stream-mechanics entries, copies the body verbatim,
// and replaces all other source entries by the caller-supplied extras.
func TestStreamWriteAtCrossFile(t *testing.T) {
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	// build a stream whose dict references another indirect dict.
	innerRef := src.Alloc()
	if err := src.Put(innerRef, pdf.Dict{"Predictor": pdf.Integer(1)}); err != nil {
		t.Fatalf("Put inner: %v", err)
	}
	body := bytes.Repeat([]byte{0xa5}, 32)
	var encoded bytes.Buffer
	zw := zlib.NewWriter(&encoded)
	zw.Write(body)
	zw.Close()
	streamRef := src.Alloc()
	w, err := src.OpenStream(streamRef, pdf.Dict{
		"Filter":      pdf.Name("FlateDecode"),
		"DecodeParms": innerRef,
		"Profile":     innerRef,
		"Tag":         pdf.Name("Demo"),
	})
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	if _, err := w.Write(encoded.Bytes()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close stream: %v", err)
	}
	src.GetMeta().Trailer["Quir:E"] = streamRef
	if err := src.Close(); err != nil {
		t.Fatalf("src.Close: %v", err)
	}

	srcX := pdf.NewExtractor(src)
	srcStream, err := pdf.CursorAt(srcX, nil).Stream(streamRef)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	s, err := ExtractStream(srcX, srcStream)
	if err != nil {
		t.Fatal(err)
	}

	dst, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(dst)
	dstRef := dst.Alloc()
	embedExtras := pdf.Dict{
		"Tag":   pdf.Name("Override"), // overrides source's Demo
		"Extra": pdf.Integer(7),       // not present in source
	}
	if _, err := rm.Embed(&streamCarrier{s: s, ref: dstRef, extras: embedExtras}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close: %v", err)
	}
	dst.GetMeta().Trailer["Quir:E"] = dstRef
	if err := dst.Close(); err != nil {
		t.Fatalf("dst.Close: %v", err)
	}

	// verify the dest stream
	dstX := pdf.NewExtractor(dst)
	dstStream, err := pdf.CursorAt(dstX, nil).Stream(dstRef)
	if err != nil {
		t.Fatalf("dst GetStream: %v", err)
	}

	// DecodeParms must survive the copy.
	dstParms, err := pdf.CursorAt(dstX, nil).Dict(dstStream.Dict["DecodeParms"])
	if err != nil {
		t.Fatalf("dst DecodeParms: %v", err)
	}
	if dstParms["Predictor"] != pdf.Integer(1) {
		t.Errorf("dst DecodeParms.Predictor = %v, want 1", dstParms["Predictor"])
	}

	// Caller's extras replace the other source entries.
	if _, ok := dstStream.Dict["Profile"]; ok {
		t.Error("dst kept source Profile entry")
	}
	if dstStream.Dict["Tag"] != pdf.Name("Override") {
		t.Errorf("dst Tag = %v, want Override", dstStream.Dict["Tag"])
	}
	if dstStream.Dict["Extra"] != pdf.Integer(7) {
		t.Errorf("dst Extra = %v, want 7", dstStream.Dict["Extra"])
	}

	// Body bytes must be unchanged.
	rc, err := pdf.DecodeStream(dst, nil, dstStream)
	if err != nil {
		t.Fatalf("DecodeStream: %v", err)
	}
	gotBody, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatalf("ReadAll body: %v", err)
	}
	if !bytes.Equal(gotBody, body) {
		t.Errorf("body mismatch: got %x..., want %x...", gotBody[:min(8, len(gotBody))], body[:8])
	}
}

// TestStreamWriteAtCrossEncryption verifies that WriteAt correctly
// transfers a stream from an encrypted source PDF into an unencrypted
// destination, producing plaintext stream bytes in the destination
// that match the original payload.  This exercises [pdf.Copier]'s
// cryptDefault recipe (decrypt-then-re-encrypt-or-pass-through).
func TestStreamWriteAtCrossEncryption(t *testing.T) {
	body := []byte("plaintext stream payload")

	// build an encrypted source PDF.
	src, srcFile := memfile.NewPDFWriter(t, pdf.V1_6, &pdf.WriterOptions{
		UserPassword:  "u",
		OwnerPassword: "o",
	})
	streamRef := src.Alloc()
	w, err := src.OpenStream(streamRef, pdf.Dict{"Tag": pdf.Name("Encrypted")})
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	if _, err := w.Write(body); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close stream: %v", err)
	}
	src.GetMeta().Trailer["Quir:E"] = streamRef
	if err := src.Close(); err != nil {
		t.Fatalf("src.Close: %v", err)
	}

	// re-open the encrypted source via a Reader that knows the password.
	r, err := pdf.NewReader(bytes.NewReader(srcFile.Data), int64(len(srcFile.Data)),
		&pdf.ReaderOptions{Password: "u"})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	srcX := pdf.NewExtractor(r)
	srcStreamRef := r.GetMeta().Trailer["Quir:E"]
	srcStream, err := pdf.CursorAt(srcX, nil).Stream(srcStreamRef)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	s, err := ExtractStream(srcX, srcStream)
	if err != nil {
		t.Fatal(err)
	}

	// embed into an unencrypted destination.
	dst, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(dst)
	dstRef := dst.Alloc()
	if _, err := rm.Embed(&streamCarrier{s: s, ref: dstRef, extras: pdf.Dict{}}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close: %v", err)
	}
	dst.GetMeta().Trailer["Quir:E"] = dstRef
	if err := dst.Close(); err != nil {
		t.Fatalf("dst.Close: %v", err)
	}

	// the destination is unencrypted; reading the stream must yield the
	// original plaintext body (decrypted from the source on the way out).
	dstX := pdf.NewExtractor(dst)
	dstStream, err := pdf.CursorAt(dstX, nil).Stream(dstRef)
	if err != nil {
		t.Fatalf("dst GetStream: %v", err)
	}
	rc, err := pdf.DecodeStream(dst, nil, dstStream)
	if err != nil {
		t.Fatalf("DecodeStream: %v", err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("dst body = %q, want %q", got, body)
	}
}

// TestStreamWriteAtRejectsForbiddenKeys verifies that WriteAt returns an
// error if the caller passes any of the stream-mechanics keys that the
// copier and writer manage themselves.
func TestStreamWriteAtRejectsForbiddenKeys(t *testing.T) {
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	streamRef := src.Alloc()
	w, err := src.OpenStream(streamRef, nil)
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	if _, err := w.Write([]byte("body")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close stream: %v", err)
	}
	src.GetMeta().Trailer["Quir:E"] = streamRef
	if err := src.Close(); err != nil {
		t.Fatalf("src.Close: %v", err)
	}

	srcX := pdf.NewExtractor(src)
	stream, err := pdf.CursorAt(srcX, nil).Stream(streamRef)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	s, err := ExtractStream(srcX, stream)
	if err != nil {
		t.Fatal(err)
	}

	forbidden := []pdf.Name{"Length", "Filter", "DecodeParms", "DP", "F", "FFilter", "FDecodeParms"}
	for _, key := range forbidden {
		dst, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
		rm := pdf.NewResourceManager(dst)
		dstRef := dst.Alloc()
		_, err := rm.Embed(&streamCarrier{
			s:      s,
			ref:    dstRef,
			extras: pdf.Dict{key: pdf.Integer(1)},
		})
		if err == nil {
			t.Errorf("WriteAt with %q in extras: want error, got nil", key)
		}
	}
}

// streamCarrier adapts a *Stream to the Embedder interface for tests
// that need to invoke WriteAt through the resource-manager flow.
type streamCarrier struct {
	s      *Stream
	ref    pdf.Reference
	extras pdf.Dict
}

func (c *streamCarrier) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := c.s.WriteAt(rm, c.ref, c.extras); err != nil {
		return nil, err
	}
	return c.ref, nil
}

// TestStreamWriteAtUnreadable checks that a stream whose /DecodeParms holds
// a reference to an unreadable object can still be copied; the reference
// then points to null in the destination (PDF 2.0, 7.3.10).
func TestStreamWriteAtUnreadable(t *testing.T) {
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	bad := src.Alloc()
	if err := src.Put(bad, pdf.Integer(1)); err != nil {
		t.Fatal(err)
	}
	streamRef := src.Alloc()
	w, err := src.OpenStream(streamRef, pdf.Dict{
		"Filter":      pdf.Array{pdf.Name("ASCIIHexDecode")},
		"DecodeParms": pdf.Array{pdf.Dict{"K": bad}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("64617461>")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	srcX := pdf.NewExtractor(unreadableGetter{Getter: src, bad: bad})
	srcStream, err := pdf.CursorAt(srcX, nil).Stream(streamRef)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ExtractStream(srcX, srcStream)
	if err != nil {
		t.Fatal(err)
	}

	dst, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(dst)
	dstRef := dst.Alloc()
	if _, err := rm.Embed(&streamCarrier{s: s, ref: dstRef, extras: pdf.Dict{}}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	c := pdf.CursorAt(pdf.NewExtractor(dst), nil)
	dstStream, err := c.Stream(dstRef)
	if err != nil {
		t.Fatal(err)
	}
	parms, err := c.Array(dstStream.Dict["DecodeParms"])
	if err != nil || len(parms) != 1 {
		t.Fatalf("DecodeParms = %v, %v", parms, err)
	}
	inner, err := c.Dict(parms[0])
	if err != nil {
		t.Fatal(err)
	}
	val, err := c.Resolve(inner["K"])
	if err != nil {
		t.Fatal(err)
	}
	if val != nil {
		t.Errorf("K resolves to %v, want null", val)
	}
}

// readTestStream writes a stream with the given dictionary and raw body to
// a PDF 1.7 file and reads it back.
func readTestStream(t *testing.T, dict pdf.Dict, body []byte) (*pdf.Extractor, *pdf.Stream) {
	t.Helper()
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	ref := src.Alloc()
	if err := src.Put(ref, pdf.NewStream(dict, body)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	src.GetMeta().Trailer["Quir:E"] = ref
	if err := src.Close(); err != nil {
		t.Fatalf("src.Close: %v", err)
	}
	srcX := pdf.NewExtractor(src)
	stream, err := pdf.CursorAt(srcX, nil).Stream(ref)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	return srcX, stream
}

// TestStreamWriteAtVersion verifies that WriteAt refuses to copy a stream
// into a file whose PDF version predates one of the stream's filters.
func TestStreamWriteAtVersion(t *testing.T) {
	cases := []struct {
		dict     pdf.Dict
		earliest pdf.Version
	}{
		{pdf.Dict{"Filter": pdf.Name("FlateDecode")}, pdf.V1_2},
		{pdf.Dict{"Filter": pdf.Array{pdf.Name("ASCIIHexDecode"), pdf.Name("JBIG2Decode")}}, pdf.V1_4},
		{pdf.Dict{"Filter": pdf.Name("JPXDecode")}, pdf.V1_5},
		{pdf.Dict{
			"Filter":      pdf.Name("FlateDecode"),
			"DecodeParms": pdf.Dict{"Predictor": pdf.Integer(12), "BitsPerComponent": pdf.Integer(16)},
		}, pdf.V1_5},
	}
	for i, c := range cases {
		srcX, stream := readTestStream(t, c.dict, []byte("data"))
		s, err := ExtractStream(srcX, stream)
		if err != nil {
			t.Fatalf("%d: ExtractStream: %v", i, err)
		}
		for _, v := range []pdf.Version{c.earliest - 1, c.earliest} {
			dst, _ := memfile.NewPDFWriter(t, v, nil)
			rm := pdf.NewResourceManager(dst)
			_, err := rm.Embed(&streamCarrier{s: s, ref: dst.Alloc()})
			if (err == nil) != (v >= c.earliest) {
				t.Errorf("%d: %s: unexpected error state %v", i, v, err)
			}
		}
	}
}

// TestExtractStreamMalformed verifies that ExtractStream rejects streams
// whose filter chain cannot be decoded.
func TestExtractStreamMalformed(t *testing.T) {
	cases := []pdf.Dict{
		{"Filter": pdf.Name("NoSuchDecode")},
		{"Filter": pdf.Array{pdf.Name("FlateDecode"), pdf.Integer(1)}},
		{"Filter": pdf.Array{pdf.Name("FlateDecode"), pdf.Name("Crypt")}},
	}
	for i, dict := range cases {
		srcX, stream := readTestStream(t, dict, []byte("data"))
		if _, err := ExtractStream(srcX, stream); err == nil {
			t.Errorf("%d: expected error, got nil", i)
		}
	}
}

// TestStreamWriteAtCryptFilterVersion verifies that a leading /Crypt
// filter does not cause WriteAt to reject a pre-1.5 destination, since the
// copier strips the crypt filter when the destination does not use crypt
// filters.
func TestStreamWriteAtCryptFilterVersion(t *testing.T) {
	body := []byte("plaintext stream payload")

	filters := []pdf.Filter{pdf.FilterCryptStandard{}, pdf.FilterCryptIdentity{}}
	for i, filter := range filters {
		src, srcFile := memfile.NewPDFWriter(t, pdf.V1_6, &pdf.WriterOptions{
			UserPassword:  "u",
			OwnerPassword: "o",
		})
		streamRef := src.Alloc()
		w, err := src.OpenStream(streamRef, nil, filter)
		if err != nil {
			t.Fatalf("%d: OpenStream: %v", i, err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatalf("%d: Write: %v", i, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("%d: close stream: %v", i, err)
		}
		src.GetMeta().Trailer["Quir:E"] = streamRef
		if err := src.Close(); err != nil {
			t.Fatalf("%d: src.Close: %v", i, err)
		}

		r, err := pdf.NewReader(bytes.NewReader(srcFile.Data), int64(len(srcFile.Data)),
			&pdf.ReaderOptions{Password: "u"})
		if err != nil {
			t.Fatalf("%d: NewReader: %v", i, err)
		}
		srcX := pdf.NewExtractor(r)
		srcStream, err := pdf.CursorAt(srcX, nil).Stream(r.GetMeta().Trailer["Quir:E"])
		if err != nil {
			t.Fatalf("%d: GetStream: %v", i, err)
		}
		if _, ok := srcStream.Dict["Filter"]; !ok {
			t.Fatalf("%d: source stream has no /Filter entry", i)
		}
		s, err := ExtractStream(srcX, srcStream)
		if err != nil {
			t.Fatalf("%d: ExtractStream: %v", i, err)
		}

		dst, _ := memfile.NewPDFWriter(t, pdf.V1_4, nil)
		rm := pdf.NewResourceManager(dst)
		dstRef := dst.Alloc()
		if _, err := rm.Embed(&streamCarrier{s: s, ref: dstRef}); err != nil {
			t.Fatalf("%d: Embed: %v", i, err)
		}
		if err := rm.Close(); err != nil {
			t.Fatalf("%d: rm.Close: %v", i, err)
		}

		dstStream, err := pdf.CursorAt(pdf.NewExtractor(dst), nil).Stream(dstRef)
		if err != nil {
			t.Fatalf("%d: dst GetStream: %v", i, err)
		}
		if f, ok := dstStream.Dict["Filter"]; ok {
			t.Errorf("%d: unexpected /Filter %v in destination", i, f)
		}
		rc, err := pdf.DecodeStream(dst, nil, dstStream)
		if err != nil {
			t.Fatalf("%d: DecodeStream: %v", i, err)
		}
		got, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("%d: ReadAll: %v", i, err)
		}
		if !bytes.Equal(got, body) {
			t.Errorf("%d: dst body = %q, want %q", i, got, body)
		}
	}
}

// TestStreamWriteAtAbbreviations verifies that WriteAt keeps the filter
// parameters of a stream which uses the abbreviated key /DP, and writes
// them under the full key.
func TestStreamWriteAtAbbreviations(t *testing.T) {
	parms := pdf.Dict{"Predictor": pdf.Integer(12), "Columns": pdf.Integer(4)}
	srcX, stream := readTestStream(t, pdf.Dict{"Filter": pdf.Name("Fl"), "DP": parms}, []byte("data"))
	s, err := ExtractStream(srcX, stream)
	if err != nil {
		t.Fatal(err)
	}

	dst, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(dst)
	dstRef := dst.Alloc()
	if _, err := rm.Embed(&streamCarrier{s: s, ref: dstRef}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close: %v", err)
	}

	dstStream, err := pdf.CursorAt(pdf.NewExtractor(dst), nil).Stream(dstRef)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	want := pdf.Dict{
		"Filter":      pdf.Name("FlateDecode"),
		"DecodeParms": parms,
	}
	if diff := cmp.Diff(want, dstStream.Dict); diff != "" {
		t.Errorf("wrong stream dict (-want +got):\n%s", diff)
	}
}
