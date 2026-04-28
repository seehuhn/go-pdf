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
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestStreamReader verifies that Reader returns the decoded body of
// the wrapped stream.
func TestStreamReader(t *testing.T) {
	src, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	if err := memfile.AddBlankPage(src); err != nil {
		t.Fatalf("AddBlankPage: %v", err)
	}
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
	stream, err := srcX.GetStream(nil, ref)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	s := ExtractStream(srcX, stream)

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

// TestStreamWriteAtCrossFile verifies that WriteAt translates internal
// references in the stream dict and copies the body verbatim, and that
// caller-supplied extras overlay the translated dict.
func TestStreamWriteAtCrossFile(t *testing.T) {
	src, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	if err := memfile.AddBlankPage(src); err != nil {
		t.Fatalf("AddBlankPage: %v", err)
	}

	// build a stream whose dict references another indirect dict.
	innerRef := src.Alloc()
	if err := src.Put(innerRef, pdf.Dict{"Y": pdf.Integer(99)}); err != nil {
		t.Fatalf("Put inner: %v", err)
	}
	body := bytes.Repeat([]byte{0xa5}, 32)
	streamRef := src.Alloc()
	w, err := src.OpenStream(streamRef, pdf.Dict{
		"Profile": innerRef,
		"Tag":     pdf.Name("Demo"),
	})
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

	srcX := pdf.NewExtractor(src)
	srcStream, err := srcX.GetStream(nil, streamRef)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	s := ExtractStream(srcX, srcStream)

	dst, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
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
	dstStream, err := dstX.GetStream(nil, dstRef)
	if err != nil {
		t.Fatalf("dst GetStream: %v", err)
	}

	// Profile reference must be translated to a fresh dest reference.
	dstProfileRef, ok := dstStream.Dict["Profile"].(pdf.Reference)
	if !ok {
		t.Fatalf("dst Profile is %T, want Reference", dstStream.Dict["Profile"])
	}
	if dstProfileRef == innerRef {
		t.Error("dst reused source Profile reference; expected translation")
	}
	dstProfile, err := dstX.GetDict(nil, dstProfileRef)
	if err != nil {
		t.Fatalf("dst Profile GetDict: %v", err)
	}
	if dstProfile["Y"] != pdf.Integer(99) {
		t.Errorf("dst Profile.Y = %v, want 99", dstProfile["Y"])
	}

	// Caller's extras overlay source dict.
	if dstStream.Dict["Tag"] != pdf.Name("Override") {
		t.Errorf("dst Tag = %v, want Override", dstStream.Dict["Tag"])
	}
	if dstStream.Dict["Extra"] != pdf.Integer(7) {
		t.Errorf("dst Extra = %v, want 7", dstStream.Dict["Extra"])
	}

	// Body bytes must be unchanged.
	rc, err := pdf.DecodeStream(dst, dstStream, 0)
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
	src, srcFile := memfile.NewPDFWriter(pdf.V1_6, &pdf.WriterOptions{
		UserPassword:  "u",
		OwnerPassword: "o",
	})
	if err := memfile.AddBlankPage(src); err != nil {
		t.Fatalf("AddBlankPage: %v", err)
	}
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
	srcStream, err := srcX.GetStream(nil, srcStreamRef)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	s := ExtractStream(srcX, srcStream)

	// embed into an unencrypted destination.
	dst, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
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
	dstStream, err := dstX.GetStream(nil, dstRef)
	if err != nil {
		t.Fatalf("dst GetStream: %v", err)
	}
	rc, err := pdf.DecodeStream(dst, dstStream, 0)
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
	src, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	if err := memfile.AddBlankPage(src); err != nil {
		t.Fatalf("AddBlankPage: %v", err)
	}
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
	stream, err := srcX.GetStream(nil, streamRef)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	s := ExtractStream(srcX, stream)

	forbidden := []pdf.Name{"Length", "Filter", "DecodeParms", "F", "FFilter", "FDecodeParms"}
	for _, key := range forbidden {
		dst, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
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
