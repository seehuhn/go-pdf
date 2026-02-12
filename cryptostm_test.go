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
	"io"
	"testing"
)

// TestEncryptedStreamMultipleReads verifies that encrypted streams can be
// read multiple times. This tests the new architecture where Stream.R contains
// the raw (encrypted) seekable data, and decryption happens lazily in
// DecodeStream.
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
		ReadPassword: func([]byte, int) string { return "user" },
	}
	r, err := NewReader(bytes.NewReader(buf.Bytes()), rOpt)
	if err != nil {
		t.Fatal(err)
	}

	// Navigate to the page's Contents stream
	pagesRef := r.GetMeta().Catalog.Pages
	pages, err := GetDict(r, pagesRef)
	if err != nil {
		t.Fatal(err)
	}
	kids, err := GetArray(r, pages["Kids"])
	if err != nil {
		t.Fatal(err)
	}
	if len(kids) == 0 {
		t.Fatal("no pages found")
	}
	page, err := GetDict(r, kids[0])
	if err != nil {
		t.Fatal(err)
	}

	// Get the Contents stream
	stream, err := GetStream(r, page["Contents"])
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
		decoded, err := DecodeStream(r, stream, 0)
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
		ReadPassword: func([]byte, int) string { return "secret" },
	}
	r, err := NewReader(bytes.NewReader(buf.Bytes()), rOpt)
	if err != nil {
		t.Fatal(err)
	}

	// Navigate to the page's Contents stream
	pagesRef := r.GetMeta().Catalog.Pages
	pages, err := GetDict(r, pagesRef)
	if err != nil {
		t.Fatal(err)
	}
	kids, err := GetArray(r, pages["Kids"])
	if err != nil {
		t.Fatal(err)
	}
	page, err := GetDict(r, kids[0])
	if err != nil {
		t.Fatal(err)
	}
	stream, err := GetStream(r, page["Contents"])
	if err != nil {
		t.Fatal(err)
	}
	if stream == nil {
		t.Fatal("stream is nil")
	}

	// Read the stream twice - the filter chain (crypt + flate) should work
	// correctly both times
	for i := range 2 {
		decoded, err := DecodeStream(r, stream, 0)
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

	r, err := NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Navigate to the page's Contents stream
	pagesRef := r.GetMeta().Catalog.Pages
	pages, err := GetDict(r, pagesRef)
	if err != nil {
		t.Fatal(err)
	}
	kids, err := GetArray(r, pages["Kids"])
	if err != nil {
		t.Fatal(err)
	}
	page, err := GetDict(r, kids[0])
	if err != nil {
		t.Fatal(err)
	}
	stream, err := GetStream(r, page["Contents"])
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
		decoded, err := DecodeStream(r, stream, 0)
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

// TestFilterCryptRoundTrip tests the Encode method of filterCrypt for symmetry.
func TestFilterCryptRoundTrip(t *testing.T) {
	testData := []byte("Test data for filter crypt encode/decode round trip.")

	// Create encryption info
	id := []byte("0123456789ABCDEF")
	sec, err := createStdSecHandler(id, "test", "test", PermAll, 128, 4)
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
	decoder, err := filter.Decode(V1_6, bytes.NewReader(encryptedBuf.Bytes()))
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
