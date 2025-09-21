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

	ref, _, err := pdf.ResourceManagerEmbed[pdf.Unused](rm, original)
	if err != nil {
		t.Fatalf("failed to embed metadata: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("failed to close resource manager: %v", err)
	}

	// Extract and compare
	extracted, err := Extract(pdfData, ref)
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
