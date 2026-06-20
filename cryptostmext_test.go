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
	"compress/zlib"
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// addContentsPage adds a minimal page tree whose single page references
// streamRef as its content stream.  Mirrors the package-internal addPage
// helper, redeclared here because external tests cannot reach it.
func addContentsPage(w *pdf.Writer, streamRef pdf.Reference) error {
	pageRef := w.Alloc()
	pagesRef := w.Alloc()
	pageDict := pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Parent":    pagesRef,
		"Resources": pdf.Dict{},
		"MediaBox":  &pdf.Rectangle{URx: 100, URy: 100},
		"Contents":  streamRef,
	}
	if err := w.Put(pageRef, pageDict); err != nil {
		return err
	}
	pagesDict := pdf.Dict{
		"Type":  pdf.Name("Pages"),
		"Kids":  pdf.Array{pageRef},
		"Count": pdf.Integer(1),
	}
	if err := w.Put(pagesRef, pagesDict); err != nil {
		return err
	}
	w.GetMeta().Catalog.Pages = pagesRef
	return nil
}

// TestIndirectFilterWriteInlines verifies that Writer.OpenStream
// resolves an indirect /Filter entry into the destination's stream
// dict so that subsequent operations (the cheap /Crypt probe,
// appendFilter for additional encoders, and the on-disk wire form)
// see direct values rather than a Reference.
func TestIndirectFilterWriteInlines(t *testing.T) {
	testData := []byte("Indirect-/Filter Identity-Crypt payload.")

	// pre-flate-compress the test data: with /Crypt /Identity at
	// position 0 the writer skips document-level encryption and the
	// on-disk bytes equal whatever we hand to OpenStream
	var flateBuf bytes.Buffer
	zw := zlib.NewWriter(&flateBuf)
	if _, err := zw.Write(testData); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	// Build an encrypted PDF using a memfile-backed writer so that
	// Writer.Get can resolve the indirect /Filter at write time.
	w, mf := memfile.NewPDFWriter(pdf.V1_6, &pdf.WriterOptions{
		UserPassword:  "u",
		OwnerPassword: "o",
	})

	filterRef := w.Alloc()
	if err := w.Put(filterRef, pdf.Array{pdf.Name("Crypt"), pdf.Name("FlateDecode")}); err != nil {
		t.Fatal(err)
	}

	streamRef := w.Alloc()
	body, err := w.OpenStream(streamRef, pdf.Dict{"Filter": filterRef})
	if err != nil {
		t.Fatalf("OpenStream with indirect /Filter: %v", err)
	}
	if _, err := body.Write(flateBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	if err := addContentsPage(w, streamRef); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(mf.Data), int64(len(mf.Data)),
		&pdf.ReaderOptions{Password: "u"})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := pdf.NewCursor(r).Stream(streamRef)
	if err != nil {
		t.Fatal(err)
	}

	// The wire form must carry a direct /Filter array â OpenStream
	// inlines the indirect reference rather than emitting it.
	filterArr, ok := stream.Dict["Filter"].(pdf.Array)
	if !ok {
		t.Fatalf("/Filter = %T, want direct Array (OpenStream should inline)", stream.Dict["Filter"])
	}
	if len(filterArr) != 2 || filterArr[0] != pdf.Name("Crypt") || filterArr[1] != pdf.Name("FlateDecode") {
		t.Errorf("/Filter = %v, want [/Crypt /FlateDecode]", filterArr)
	}

	decoded, err := pdf.DecodeStream(r, nil, stream)
	if err != nil {
		t.Fatalf("DecodeStream: %v", err)
	}
	got, err := io.ReadAll(decoded)
	if err != nil {
		t.Fatal(err)
	}
	decoded.Close()
	if !bytes.Equal(got, testData) {
		t.Errorf("DecodeStream: got %q, want %q", got, testData)
	}
}

// TestIndirectFilterReadResolves verifies that DecodeStream and
// RawStreamReader correctly classify a stream whose on-disk /Filter
// is an indirect reference.  Such a wire form cannot be produced by
// OpenStream (which inlines), so we synthesise it by mutating the
// in-memory dict of an already-parsed stream.
func TestIndirectFilterReadResolves(t *testing.T) {
	testData := []byte("Indirect-/Filter on-disk payload.")

	var flateBuf bytes.Buffer
	zw := zlib.NewWriter(&flateBuf)
	if _, err := zw.Write(testData); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	w, mf := memfile.NewPDFWriter(pdf.V1_6, &pdf.WriterOptions{
		UserPassword:  "u",
		OwnerPassword: "o",
	})

	// Write the filter array as an indirect object, plus a stream
	// whose /Filter array is direct so OpenStream produces well-formed
	// on-disk bytes.  We will redirect the in-memory dict to the
	// indirect array after reading.
	filterRef := w.Alloc()
	if err := w.Put(filterRef, pdf.Array{pdf.Name("Crypt"), pdf.Name("FlateDecode")}); err != nil {
		t.Fatal(err)
	}
	streamRef := w.Alloc()
	body, err := w.OpenStream(streamRef, pdf.Dict{
		"Filter": pdf.Array{pdf.Name("Crypt"), pdf.Name("FlateDecode")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := body.Write(flateBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	if err := addContentsPage(w, streamRef); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(mf.Data), int64(len(mf.Data)),
		&pdf.ReaderOptions{Password: "u"})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := pdf.NewCursor(r).Stream(streamRef)
	if err != nil {
		t.Fatal(err)
	}

	// Replace the parsed direct /Filter with the indirect reference,
	// so the read paths must resolve it to classify the stream.
	stream.Dict["Filter"] = filterRef

	decoded, err := pdf.DecodeStream(r, nil, stream)
	if err != nil {
		t.Fatalf("DecodeStream: %v", err)
	}
	got, err := io.ReadAll(decoded)
	if err != nil {
		t.Fatal(err)
	}
	decoded.Close()
	if !bytes.Equal(got, testData) {
		t.Errorf("DecodeStream: got %q, want %q", got, testData)
	}

	raw, err := pdf.RawStreamReader(r, stream)
	if err != nil {
		t.Fatalf("RawStreamReader: %v", err)
	}
	rawBytes, err := io.ReadAll(raw)
	if err != nil {
		t.Fatal(err)
	}
	raw.Close()
	if !bytes.Equal(rawBytes, flateBuf.Bytes()) {
		t.Errorf("RawStreamReader bytes do not match the on-disk flate payload")
	}
}

// TestCopyInlinesIndirectFilter verifies that the Copier resolves
// indirect /Filter (and /DecodeParms) entries when copying a stream,
// so that the destination dict carries direct values.  This keeps the
// destination self-contained and lets non-seekable writers avoid the
// indirect-resolution path in Writer.OpenStream.
func TestCopyInlinesIndirectFilter(t *testing.T) {
	testData := []byte("Indirect-/Filter copy-inlining payload.")

	var flateBuf bytes.Buffer
	zw := zlib.NewWriter(&flateBuf)
	if _, err := zw.Write(testData); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	// Source: encrypted PDF with /Filter as an indirect reference.
	srcW, srcMF := memfile.NewPDFWriter(pdf.V1_6, &pdf.WriterOptions{
		UserPassword:  "src",
		OwnerPassword: "src",
	})
	srcFilterRef := srcW.Alloc()
	if err := srcW.Put(srcFilterRef, pdf.Array{pdf.Name("Crypt"), pdf.Name("FlateDecode")}); err != nil {
		t.Fatal(err)
	}
	srcStreamRef := srcW.Alloc()
	body, err := srcW.OpenStream(srcStreamRef, pdf.Dict{"Filter": srcFilterRef})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := body.Write(flateBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	if err := addContentsPage(srcW, srcStreamRef); err != nil {
		t.Fatal(err)
	}
	if err := srcW.Close(); err != nil {
		t.Fatal(err)
	}

	srcR, err := pdf.NewReader(bytes.NewReader(srcMF.Data), int64(len(srcMF.Data)),
		&pdf.ReaderOptions{Password: "src"})
	if err != nil {
		t.Fatal(err)
	}

	// Destination: a different encrypted PDF.  bytes.Buffer is fine â
	// the Copier inlines /Filter so the writer's cheap probe never
	// needs to call Resolve on the destination.
	dstBuf := &bytes.Buffer{}
	dstW, err := pdf.NewWriter(dstBuf, pdf.V1_6, &pdf.WriterOptions{
		UserPassword:  "dst",
		OwnerPassword: "dst",
	})
	if err != nil {
		t.Fatal(err)
	}
	copier := pdf.NewCopier(dstW, srcR)
	dstStreamRef, err := copier.CopyReference(srcStreamRef)
	if err != nil {
		t.Fatalf("CopyReference: %v", err)
	}
	if err := addContentsPage(dstW, dstStreamRef); err != nil {
		t.Fatal(err)
	}
	if err := dstW.Close(); err != nil {
		t.Fatal(err)
	}

	dstR, err := pdf.NewReader(bytes.NewReader(dstBuf.Bytes()), int64(dstBuf.Len()),
		&pdf.ReaderOptions{Password: "dst"})
	if err != nil {
		t.Fatal(err)
	}
	dstStream, err := pdf.NewCursor(dstR).Stream(dstStreamRef)
	if err != nil {
		t.Fatal(err)
	}

	// The destination's /Filter must be a direct array, not a Reference.
	filterArr, ok := dstStream.Dict["Filter"].(pdf.Array)
	if !ok {
		t.Fatalf("destination /Filter = %T, want direct Array", dstStream.Dict["Filter"])
	}
	if len(filterArr) != 2 || filterArr[0] != pdf.Name("Crypt") || filterArr[1] != pdf.Name("FlateDecode") {
		t.Errorf("destination /Filter = %v, want [/Crypt /FlateDecode]", filterArr)
	}

	decoded, err := pdf.DecodeStream(dstR, nil, dstStream)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(decoded)
	if err != nil {
		t.Fatal(err)
	}
	decoded.Close()
	if !bytes.Equal(got, testData) {
		t.Errorf("copied stream: got %q, want %q", got, testData)
	}
}
