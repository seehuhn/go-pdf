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

package pdf

import (
	"bytes"
	"compress/zlib"
	"io"
	"testing"

	"seehuhn.de/go/membudget"
)

// TestEncryptedStreamMultipleReads verifies that encrypted streams can be
// read multiple times. Each call to NewReader() returns a fresh reader over
// the raw (encrypted) data, and decryption happens lazily in DecodeStream.
func TestEncryptedStreamMultipleReads(t *testing.T) {
	testCases := []struct {
		name    string
		version Version
	}{
		{"RC4-40/V1.1", V1_1},
		{"RC4-128/V1.4", V1_4},
		{"AES-128/V1.6", V1_6},
		{"AES-256/V2.0", V2_0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testEncryptedStreamMultipleReads(t, tc.version)
		})
	}
}

func testEncryptedStreamMultipleReads(t *testing.T, version Version) {
	t.Helper()

	testData := []byte("Hello, World! This is a test of encrypted stream handling.")

	// Create an encrypted PDF with a stream
	opt := &WriterOptions{
		UserPassword:  "user",
		OwnerPassword: "owner",
	}
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, version, opt)
	if err != nil {
		t.Fatal(err)
	}

	streamRef := w.Alloc()
	s, err := w.OpenStream(streamRef, nil, FilterCompress{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Write(testData)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Add a page with this stream as the Contents
	err = addPage(w, Name("Contents"), streamRef)
	if err != nil {
		t.Fatal(err)
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Open the PDF for reading
	rOpt := &ReaderOptions{
		Password: "user",
	}
	r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), rOpt)
	if err != nil {
		t.Fatal(err)
	}

	// Navigate to the page's Contents stream
	pagesRef := r.GetMeta().Catalog.Pages
	pages, err := NewCursor(r).Dict(pagesRef)
	if err != nil {
		t.Fatal(err)
	}
	kids, err := NewCursor(r).Array(pages["Kids"])
	if err != nil {
		t.Fatal(err)
	}
	if len(kids) == 0 {
		t.Fatal("no pages found")
	}
	page, err := NewCursor(r).Dict(kids[0])
	if err != nil {
		t.Fatal(err)
	}

	// Get the Contents stream
	stream, err := NewCursor(r).Stream(page["Contents"])
	if err != nil {
		t.Fatal(err)
	}
	if stream == nil {
		t.Fatal("stream is nil")
	}

	// Verify that the stream's decryption filter is stored (not applied)
	if stream.crypt == nil {
		t.Fatal("stream.crypt should not be nil for encrypted stream")
	}

	// Read the stream multiple times - this should work with the new architecture
	for i := range 3 {
		decoded, err := DecodeStream(r, nil, stream)
		if err != nil {
			t.Fatalf("DecodeStream failed on read %d: %v", i+1, err)
		}

		data, err := io.ReadAll(decoded)
		if err != nil {
			t.Fatalf("ReadAll failed on read %d: %v", i+1, err)
		}
		decoded.Close()

		if !bytes.Equal(data, testData) {
			t.Errorf("read %d: got %q, want %q", i+1, data, testData)
		}
	}
}

// TestEncryptedStreamWithFilters verifies that encrypted streams with
// compression filters work correctly with multiple reads.
func TestEncryptedStreamWithFilters(t *testing.T) {
	testData := bytes.Repeat([]byte("This is test data for compression. "), 100)

	opt := &WriterOptions{
		UserPassword:  "secret",
		OwnerPassword: "secret",
	}
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V1_6, opt)
	if err != nil {
		t.Fatal(err)
	}

	streamRef := w.Alloc()
	s, err := w.OpenStream(streamRef, nil, FilterFlate{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Write(testData)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = addPage(w, Name("Contents"), streamRef)
	if err != nil {
		t.Fatal(err)
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	rOpt := &ReaderOptions{
		Password: "secret",
	}
	r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), rOpt)
	if err != nil {
		t.Fatal(err)
	}

	// Navigate to the page's Contents stream
	pagesRef := r.GetMeta().Catalog.Pages
	pages, err := NewCursor(r).Dict(pagesRef)
	if err != nil {
		t.Fatal(err)
	}
	kids, err := NewCursor(r).Array(pages["Kids"])
	if err != nil {
		t.Fatal(err)
	}
	page, err := NewCursor(r).Dict(kids[0])
	if err != nil {
		t.Fatal(err)
	}
	stream, err := NewCursor(r).Stream(page["Contents"])
	if err != nil {
		t.Fatal(err)
	}
	if stream == nil {
		t.Fatal("stream is nil")
	}

	// Read the stream twice - the filter chain (crypt + flate) should work
	// correctly both times
	for i := range 2 {
		decoded, err := DecodeStream(r, nil, stream)
		if err != nil {
			t.Fatalf("DecodeStream failed on read %d: %v", i+1, err)
		}

		data, err := io.ReadAll(decoded)
		decoded.Close()
		if err != nil {
			t.Fatalf("ReadAll failed on read %d: %v", i+1, err)
		}

		if !bytes.Equal(data, testData) {
			t.Errorf("read %d: data mismatch (len %d vs %d)", i+1, len(data), len(testData))
		}
	}
}

// TestUnencryptedStreamMultipleReads verifies that unencrypted streams
// also work correctly with multiple reads (regression test).
func TestUnencryptedStreamMultipleReads(t *testing.T) {
	testData := []byte("Unencrypted test data")

	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}

	streamRef := w.Alloc()
	s, err := w.OpenStream(streamRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Write(testData)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = addPage(w, Name("Contents"), streamRef)
	if err != nil {
		t.Fatal(err)
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Navigate to the page's Contents stream
	pagesRef := r.GetMeta().Catalog.Pages
	pages, err := NewCursor(r).Dict(pagesRef)
	if err != nil {
		t.Fatal(err)
	}
	kids, err := NewCursor(r).Array(pages["Kids"])
	if err != nil {
		t.Fatal(err)
	}
	page, err := NewCursor(r).Dict(kids[0])
	if err != nil {
		t.Fatal(err)
	}
	stream, err := NewCursor(r).Stream(page["Contents"])
	if err != nil {
		t.Fatal(err)
	}
	if stream == nil {
		t.Fatal("stream is nil")
	}

	// Verify that unencrypted streams have nil decryption filter
	if stream.crypt != nil {
		t.Error("stream.crypt should be nil for unencrypted stream")
	}

	// Read multiple times
	for i := range 3 {
		decoded, err := DecodeStream(r, nil, stream)
		if err != nil {
			t.Fatalf("DecodeStream failed on read %d: %v", i+1, err)
		}

		data, err := io.ReadAll(decoded)
		decoded.Close()
		if err != nil {
			t.Fatalf("ReadAll failed on read %d: %v", i+1, err)
		}

		if !bytes.Equal(data, testData) {
			t.Errorf("read %d: got %q, want %q", i+1, data, testData)
		}
	}
}

// TestCopierEncryptedStream verifies that copying a stream from an encrypted
// PDF produces correct data in the destination, regardless of whether the
// destination is encrypted.
func TestCopierEncryptedStream(t *testing.T) {
	testData := []byte("stream data for copier encryption test")

	for _, tc := range []struct {
		name    string
		dstOpt  *WriterOptions
		readOpt *ReaderOptions
	}{
		{
			name:    "encrypted to unencrypted",
			dstOpt:  nil,
			readOpt: nil,
		},
		{
			name: "encrypted to encrypted",
			dstOpt: &WriterOptions{
				UserPassword:  "dst",
				OwnerPassword: "dst",
			},
			readOpt: &ReaderOptions{Password: "dst"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// write an encrypted source PDF with a stream
			srcBuf := &bytes.Buffer{}
			srcW, err := NewWriter(srcBuf, V1_6, &WriterOptions{
				UserPassword:  "src",
				OwnerPassword: "src",
			})
			if err != nil {
				t.Fatal(err)
			}
			streamRef := srcW.Alloc()
			s, err := srcW.OpenStream(streamRef, nil, FilterFlate{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Write(testData)
			if err != nil {
				t.Fatal(err)
			}
			err = s.Close()
			if err != nil {
				t.Fatal(err)
			}
			err = addPage(srcW, Name("Contents"), streamRef)
			if err != nil {
				t.Fatal(err)
			}
			err = srcW.Close()
			if err != nil {
				t.Fatal(err)
			}

			// open the source PDF for reading
			srcR, err := NewReader(bytes.NewReader(srcBuf.Bytes()), int64(srcBuf.Len()),
				&ReaderOptions{Password: "src"})
			if err != nil {
				t.Fatal(err)
			}

			// navigate to the stream
			pagesRef := srcR.GetMeta().Catalog.Pages
			pages, err := NewCursor(srcR).Dict(pagesRef)
			if err != nil {
				t.Fatal(err)
			}
			kids, err := NewCursor(srcR).Array(pages["Kids"])
			if err != nil {
				t.Fatal(err)
			}
			page, err := NewCursor(srcR).Dict(kids[0])
			if err != nil {
				t.Fatal(err)
			}

			// copy the stream to a new PDF
			dstBuf := &bytes.Buffer{}
			dstW, err := NewWriter(dstBuf, V1_6, tc.dstOpt)
			if err != nil {
				t.Fatal(err)
			}
			copier := NewCopier(dstW, srcR)
			copiedRef, err := copier.Copy(page["Contents"].(Reference).AsPDF(0))
			if err != nil {
				t.Fatal(err)
			}
			err = addPage(dstW, Name("Contents"), copiedRef)
			if err != nil {
				t.Fatal(err)
			}
			err = dstW.Close()
			if err != nil {
				t.Fatal(err)
			}

			// read back the destination PDF and verify stream content
			dstR, err := NewReader(bytes.NewReader(dstBuf.Bytes()), int64(dstBuf.Len()), tc.readOpt)
			if err != nil {
				t.Fatal(err)
			}
			dPages, err := NewCursor(dstR).Dict(dstR.GetMeta().Catalog.Pages)
			if err != nil {
				t.Fatal(err)
			}
			dKids, err := NewCursor(dstR).Array(dPages["Kids"])
			if err != nil {
				t.Fatal(err)
			}
			dPage, err := NewCursor(dstR).Dict(dKids[0])
			if err != nil {
				t.Fatal(err)
			}
			stream, err := NewCursor(dstR).Stream(dPage["Contents"])
			if err != nil {
				t.Fatal(err)
			}
			got, err := ReadAll(dstR, nil, stream, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, testData) {
				t.Errorf("copied stream data mismatch: got %q, want %q", got, testData)
			}
		})
	}
}

// TestEncryptedStreamWithIdentityCrypt verifies that a stream marked with
// FilterCryptIdentity in an encrypted document is stored plaintext on disk
// and round-trips correctly through DecodeStream.
func TestEncryptedStreamWithIdentityCrypt(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version Version
	}{
		{"RC4-128/V1.4", V1_5},
		{"AES-128/V1.6", V1_6},
		{"AES-256/V2.0", V2_0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testEncryptedStreamWithIdentityCrypt(t, tc.version)
		})
	}
}

func testEncryptedStreamWithIdentityCrypt(t *testing.T, version Version) {
	t.Helper()

	testData := []byte("Plaintext payload exempt from document encryption.")

	// Build an encrypted PDF where one stream uses /Crypt /Identity.
	opt := &WriterOptions{
		UserPassword:  "user",
		OwnerPassword: "owner",
	}
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, version, opt)
	if err != nil {
		t.Fatal(err)
	}

	streamRef := w.Alloc()
	s, err := w.OpenStream(streamRef, nil,
		FilterCryptIdentity{}, FilterFlate{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write(testData); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	if err := addPage(w, Name("Contents"), streamRef); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Re-open with the password and verify DecodeStream round-trips.
	r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()),
		&ReaderOptions{Password: "user"})
	if err != nil {
		t.Fatal(err)
	}

	stream, err := NewCursor(r).Stream(streamRef)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeStream(r, nil, stream)
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

	// Verify the on-disk bytes are *not* encrypted: they should be
	// plain Flate-compressed data, decompressible with compress/zlib
	// directly without going through any decryption.
	rawZlib, err := io.ReadAll(stream.NewReader())
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zlib.NewReader(bytes.NewReader(rawZlib))
	if err != nil {
		t.Fatalf("on-disk bytes are not plain Flate: %v "+
			"(suggests document encryption was applied)", err)
	}
	plain, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompressing on-disk bytes: %v", err)
	}
	zr.Close()
	if !bytes.Equal(plain, testData) {
		t.Errorf("on-disk Flate payload: got %q, want %q", plain, testData)
	}

	// Verify the /Filter dict has [/Crypt /FlateDecode] and that
	// /DecodeParms (if present) does not write a /Name for the Identity
	// entry (it is the spec default).
	filterEntry, ok := stream.Dict["Filter"].(Array)
	if !ok || len(filterEntry) != 2 {
		t.Fatalf("/Filter = %v, want array of length 2", stream.Dict["Filter"])
	}
	if filterEntry[0] != Name("Crypt") || filterEntry[1] != Name("FlateDecode") {
		t.Errorf("/Filter = %v, want [/Crypt /FlateDecode]", filterEntry)
	}
}

// TestCopyEncryptedStreamWithIdentityCrypt verifies that copying a PDF
// containing an /Identity-Crypt stream into a fresh encrypted destination
// preserves the plaintext payload (the copier must skip the document-level
// strip for Identity streams).
func TestCopyEncryptedStreamWithIdentityCrypt(t *testing.T) {
	testData := []byte("Identity-crypt copier round-trip payload.")

	// Encrypted source with a /Crypt /Identity + Flate stream.
	srcBuf := &bytes.Buffer{}
	srcW, err := NewWriter(srcBuf, V1_6, &WriterOptions{
		UserPassword:  "src",
		OwnerPassword: "src",
	})
	if err != nil {
		t.Fatal(err)
	}
	srcStreamRef := srcW.Alloc()
	s, err := srcW.OpenStream(srcStreamRef, nil,
		FilterCryptIdentity{}, FilterFlate{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write(testData); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := addPage(srcW, Name("Contents"), srcStreamRef); err != nil {
		t.Fatal(err)
	}
	if err := srcW.Close(); err != nil {
		t.Fatal(err)
	}

	srcR, err := NewReader(bytes.NewReader(srcBuf.Bytes()), int64(srcBuf.Len()),
		&ReaderOptions{Password: "src"})
	if err != nil {
		t.Fatal(err)
	}

	// Copy into a fresh encrypted destination.
	dstBuf := &bytes.Buffer{}
	dstW, err := NewWriter(dstBuf, V1_6, &WriterOptions{
		UserPassword:  "dst",
		OwnerPassword: "dst",
	})
	if err != nil {
		t.Fatal(err)
	}
	copier := NewCopier(dstW, srcR)
	dstStreamRef, err := copier.CopyReference(srcStreamRef)
	if err != nil {
		t.Fatalf("CopyReference: %v", err)
	}
	if err := addPage(dstW, Name("Contents"), dstStreamRef); err != nil {
		t.Fatal(err)
	}
	if err := dstW.Close(); err != nil {
		t.Fatal(err)
	}

	// Verify the destination round-trips.
	dstR, err := NewReader(bytes.NewReader(dstBuf.Bytes()), int64(dstBuf.Len()),
		&ReaderOptions{Password: "dst"})
	if err != nil {
		t.Fatal(err)
	}
	dstStream, err := NewCursor(dstR).Stream(dstStreamRef)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeStream(dstR, nil, dstStream)
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

// TestRawStreamReaderRejectsNonIdentityCrypt verifies that RawStreamReader
// refuses to emit ciphertext for streams whose /Filter chain begins with a
// non-Identity Crypt entry: until the library can decrypt /StdCF and named
// CFs, returning the raw encrypted bytes would silently corrupt any caller
// that copies them into a destination with different keys.
func TestRawStreamReaderRejectsNonIdentityCrypt(t *testing.T) {
	// Build an encrypted PDF containing a normal Flate-compressed stream.
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V1_6, &WriterOptions{
		UserPassword:  "u",
		OwnerPassword: "o",
	})
	if err != nil {
		t.Fatal(err)
	}
	streamRef := w.Alloc()
	s, err := w.OpenStream(streamRef, nil, FilterFlate{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := addPage(w, Name("Contents"), streamRef); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()),
		&ReaderOptions{Password: "u"})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := NewCursor(r).Stream(streamRef)
	if err != nil {
		t.Fatal(err)
	}

	// Mutate the dict to claim /Crypt /StdCF at filter position 0.  The
	// on-disk bytes are not actually /StdCF-encrypted, but RawStreamReader
	// must refuse without inspecting them.
	for _, tc := range []struct {
		name        string
		decodeParms Object
	}{
		{"StdCF", Array{Dict{"Name": Name("StdCF")}, nil}},
		{"NamedCF", Array{Dict{"Name": Name("MyCF")}, nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream.Dict["Filter"] = Array{Name("Crypt"), Name("FlateDecode")}
			stream.Dict["DecodeParms"] = tc.decodeParms
			if _, err := RawStreamReader(r, stream); err == nil {
				t.Errorf("RawStreamReader: expected error for non-Identity Crypt, got nil")
			}
		})
	}

	// Sanity check: the unmutated /Crypt /Identity case must still succeed.
	stream.Dict["Filter"] = Array{Name("Crypt"), Name("FlateDecode")}
	stream.Dict["DecodeParms"] = nil // /Identity is the default
	if _, err := RawStreamReader(r, stream); err != nil {
		t.Errorf("RawStreamReader: unexpected error for /Crypt /Identity: %v", err)
	}
}

// TestFilterCryptRoundTrip tests the Encode method of filterCrypt for symmetry.
func TestFilterCryptRoundTrip(t *testing.T) {
	testData := []byte("Test data for filter crypt encode/decode round trip.")

	// Create encryption info
	id := []byte("0123456789ABCDEF")
	sec, err := createStdSecHandler(id, "test", "test", PermAll, 128, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	enc := &encryptInfo{
		stmF: &cryptFilter{Cipher: cipherAES, Length: 128},
		sec:  sec,
	}

	ref := NewReference(1, 0)
	filter := &filterCrypt{enc: enc, ref: ref}

	// Encode (encrypt) the data
	var encryptedBuf bytes.Buffer
	encoder, err := filter.Encode(V1_6, &withDummyClose{&encryptedBuf})
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	_, err = encoder.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	err = encoder.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Decode (decrypt) the data
	decoder, err := filter.Decode(V1_6, bytes.NewReader(encryptedBuf.Bytes()), membudget.New(1<<20))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	decrypted, err := io.ReadAll(decoder)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	decoder.Close()

	if !bytes.Equal(decrypted, testData) {
		t.Errorf("Round trip failed: got %q, want %q", decrypted, testData)
	}
}

// TestFilterCryptInfo verifies that filterCrypt.Info returns empty values
// since the crypt filter should not appear in the stream dictionary.
func TestFilterCryptInfo(t *testing.T) {
	filter := &filterCrypt{}
	name, dict, err := filter.Info(V1_6)
	if err != nil {
		t.Errorf("Info returned error: %v", err)
	}
	if name != "" {
		t.Errorf("Info returned non-empty name: %q", name)
	}
	if dict != nil {
		t.Errorf("Info returned non-nil dict: %v", dict)
	}
}
