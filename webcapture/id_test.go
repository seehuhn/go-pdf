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

package webcapture

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var testCases = []struct {
	name    string
	version pdf.Version
	data    *Identifier
}{
	{
		name:    "zero hash",
		version: pdf.V1_7,
		data:    &Identifier{ID: make([]byte, 16)},
	},
	{
		name:    "all ones",
		version: pdf.V1_7,
		data:    &Identifier{ID: bytes.Repeat([]byte{0xFF}, 16)},
	},
	{
		name:    "sequential bytes",
		version: pdf.V1_7,
		data:    &Identifier{ID: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}},
	},
	{
		name:    "typical MD5 hash",
		version: pdf.V1_7,
		data:    &Identifier{ID: []byte{0xd4, 0x1d, 0x8c, 0xd9, 0x8f, 0x00, 0xb2, 0x04, 0xe9, 0x80, 0x09, 0x98, 0xec, 0xf8, 0x42, 0x7e}},
	},
	{
		name:    "PDF 2.0",
		version: pdf.V2_0,
		data:    &Identifier{ID: []byte{0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89}},
	},
	{
		name:    "single use",
		version: pdf.V1_7,
		data:    &Identifier{ID: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, SingleUse: true},
	},
}

func roundTripTest(t *testing.T, version pdf.Version, id *Identifier) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	obj, err := rm.Embed(id)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("embed failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := ExtractIdentifier(x, obj)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	// compare only ID field, ignore SingleUse (not stored in PDF)
	if diff := cmp.Diff(id.ID, decoded.ID); diff != "" {
		t.Errorf("round-trip failed (-want +got):\n%s", diff)
	}
}

func TestIdentifierRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripTest(t, tc.version, tc.data)
		})
	}
}

func TestExtractIdentifierNil(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	id, err := ExtractIdentifier(x, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != nil {
		t.Errorf("expected nil, got %v", id)
	}
}

func TestExtractIdentifierWrongLength(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	// too short
	id, err := ExtractIdentifier(x, pdf.String([]byte{1, 2, 3}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != nil {
		t.Errorf("expected nil for short string, got %v", id)
	}

	// too long
	id, err = ExtractIdentifier(x, pdf.String(make([]byte, 32)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != nil {
		t.Errorf("expected nil for long string, got %v", id)
	}
}

func TestEmbedIdentifierVersionCheck(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_2, nil)
	rm := pdf.NewResourceManager(w)

	id := &Identifier{ID: make([]byte, 16)}
	_, err := rm.Embed(id)
	if err == nil {
		t.Error("expected version error for PDF 1.2, got nil")
	}
}

func TestEmbedIdentifierInvalidLength(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	id := &Identifier{ID: []byte{1, 2, 3}} // too short
	_, err := rm.Embed(id)
	if err == nil {
		t.Error("expected error for invalid length, got nil")
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		rm := pdf.NewResourceManager(w)
		obj, err := rm.Embed(tc.data)
		if err != nil {
			continue
		}
		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = obj
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		id, err := ExtractIdentifier(x, obj)
		if err != nil {
			t.Skip("malformed identifier")
		}
		if id == nil {
			t.Skip("nil identifier")
		}

		roundTripTest(t, pdf.GetVersion(r), id)
	})
}
