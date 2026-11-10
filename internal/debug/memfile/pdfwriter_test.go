// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package memfile

import (
	"io"
	"testing"

	"seehuhn.de/go/pdf"
)

// TestWriterReadAfterClose verifies that a memfile Writer can be used for
// reading after it is closed, specifically testing stream operations.
func TestWriterReadAfterClose(t *testing.T) {
	writer, _ := NewPDFWriter(t, pdf.V2_0, nil)

	// Create a stream containing "hello world"
	content := "hello world"
	streamDict := pdf.Dict{
		"Length": pdf.Integer(len(content)),
	}
	stream := pdf.NewStream(streamDict, []byte(content))

	// Write the stream to the PDF
	ref := writer.Alloc()
	err := writer.Put(ref, stream)
	if err != nil {
		t.Fatal(err)
	}

	// Close the writer
	err = writer.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Now try to read the stream back from the closed writer
	streamObj, err := pdf.NewCursor(writer).Stream(ref)
	if err != nil {
		t.Fatal(err)
	}
	if streamObj == nil {
		t.Fatal("stream object is nil")
	}

	// Decode and read the stream content
	stm, err := pdf.DecodeStream(writer, nil, streamObj)
	if err != nil {
		t.Fatal(err)
	}

	// Read the content back
	readContent, err := io.ReadAll(stm)
	if err != nil {
		t.Fatal(err)
	}

	// Verify we got back "hello world"
	if string(readContent) != content {
		t.Errorf("content mismatch: got %q, want %q", string(readContent), content)
	}
}
