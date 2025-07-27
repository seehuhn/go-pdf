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

package memfile

import (
	"io"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
)

// TestWriterReadAfterClose verifies that a memfile Writer can be used for
// reading after it is closed, specifically testing stream operations.
func TestWriterReadAfterClose(t *testing.T) {
	writer, _ := NewPDFWriter(pdf.V2_0, nil)

	// Create a stream containing "hello world"
	content := "hello world"
	streamDict := pdf.Dict{
		"Length": pdf.Integer(len(content)),
	}
	stream := &pdf.Stream{
		Dict: streamDict,
		R:    strings.NewReader(content),
	}

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
	streamObj, err := pdf.GetStream(writer, ref)
	if err != nil {
		t.Fatal(err)
	}
	if streamObj == nil {
		t.Fatal("stream object is nil")
	}

	// Decode and read the stream content
	stm, err := pdf.DecodeStream(writer, streamObj, 0)
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
