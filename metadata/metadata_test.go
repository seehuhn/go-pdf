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

package metadata

import (
	"bytes"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/xmp"
)

func TestRoundTrip(t *testing.T) {
	// Create XMP packet with Dublin Core properties
	packet := xmp.NewPacket()
	dc := &xmp.DublinCore{}
	dc.Title.Set(language.Und, "Test Document")
	dc.Creator.Append(xmp.NewProperName("Test Author"))

	err := packet.Set(dc)
	if err != nil {
		t.Fatalf("failed to set properties: %v", err)
	}

	original := &Stream{Data: packet}

	// Create in-memory PDF and embed metadata
	pdfData, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(pdfData)

	ref, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("failed to embed metadata: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("failed to close resource manager: %v", err)
	}

	// Extract and compare
	extracted, err := Extract(pdf.NewExtractor(pdfData), nil, ref, false)
	if err != nil {
		t.Fatalf("failed to extract metadata: %v", err)
	}

	var originalDC, extractedDC xmp.DublinCore
	original.Data.Get(&originalDC)
	extracted.Data.Get(&extractedDC)

	if diff := cmp.Diff(extractedDC, originalDC); diff != "" {
		t.Errorf("round trip failed (-got +want):\n%s", diff)
	}
}

func TestRoundTripPadded(t *testing.T) {
	const padTo = 4096

	packet := xmp.NewPacket()
	dc := &xmp.DublinCore{}
	dc.Title.Set(language.Und, "Padded Test Document")
	dc.Creator.Append(xmp.NewProperName("Test Author"))
	if err := packet.Set(dc); err != nil {
		t.Fatalf("failed to set properties: %v", err)
	}

	original := &Stream{Data: packet, PadToLength: padTo}

	pdfData, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(pdfData)
	ref, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("failed to embed metadata: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("failed to close resource manager: %v", err)
	}

	// round-trip via Extract
	extracted, err := Extract(pdf.NewExtractor(pdfData), nil, ref, false)
	if err != nil {
		t.Fatalf("failed to extract metadata: %v", err)
	}
	var originalDC, extractedDC xmp.DublinCore
	original.Data.Get(&originalDC)
	extracted.Data.Get(&extractedDC)
	if diff := cmp.Diff(extractedDC, originalDC); diff != "" {
		t.Errorf("round trip failed (-got +want):\n%s", diff)
	}

	// inspect the raw stream: on-disk length and trailer marker
	stream, err := pdf.GetStream(pdfData, ref)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	body, err := pdf.DecodeStream(pdfData, stream, 0)
	if err != nil {
		t.Fatalf("DecodeStream: %v", err)
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if len(raw) != padTo {
		t.Errorf("on-disk length: got %d, want %d", len(raw), padTo)
	}
	if !bytes.Contains(raw, []byte(`<?xpacket end="w"?>`)) {
		t.Errorf("trailer is not the writable form: %q",
			tail(raw, 32))
	}
}

func TestRoundTripUnpaddedTrailer(t *testing.T) {
	packet := xmp.NewPacket()
	dc := &xmp.DublinCore{}
	dc.Title.Set(language.Und, "Read-only")
	if err := packet.Set(dc); err != nil {
		t.Fatalf("failed to set properties: %v", err)
	}

	original := &Stream{Data: packet}

	pdfData, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(pdfData)
	ref, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("failed to embed metadata: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("failed to close resource manager: %v", err)
	}

	stream, err := pdf.GetStream(pdfData, ref)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	body, err := pdf.DecodeStream(pdfData, stream, 0)
	if err != nil {
		t.Fatalf("DecodeStream: %v", err)
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if !bytes.Contains(raw, []byte(`<?xpacket end="r"?>`)) {
		t.Errorf("trailer is not the read-only form: %q", tail(raw, 32))
	}
}

func tail(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[len(b)-n:]
}
