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

package file

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestEncryptedPayloadRoundTrip(t *testing.T) {
	testCases := []struct {
		name string
		ep   *EncryptedPayload
	}{
		{
			name: "basic encrypted payload",
			ep: &EncryptedPayload{
				FilterName: "AcmeCustomCrypto",
				Version:    "1.0",
			},
		},
		{
			name: "encrypted payload without version",
			ep: &EncryptedPayload{
				FilterName: "MySecurityHandler",
			},
		},
		{
			name: "encrypted payload with complex version",
			ep: &EncryptedPayload{
				FilterName: "AdvancedCrypto",
				Version:    "2.1.3",
			},
		},
		{
			name: "single use encrypted payload",
			ep: &EncryptedPayload{
				FilterName: "TestCrypto",
				Version:    "1.5",
				SingleUse:  true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test both SingleUse true and false
			for _, singleUse := range []bool{false, true} {
				t.Run(func(name string) string {
					if singleUse {
						return name + "_single_use"
					}
					return name + "_indirect"
				}(tc.name), func(t *testing.T) {
					original := *tc.ep
					original.SingleUse = singleUse

					// Round trip test
					buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
					rm := pdf.NewResourceManager(buf)

					// Embed the encrypted payload
					obj, _, err := original.Embed(rm)
					if err != nil {
						t.Fatal(err)
					}

					err = rm.Close()
					if err != nil {
						t.Fatal(err)
					}
					err = buf.Close()
					if err != nil {
						t.Fatal(err)
					}

					// Extract it back
					x := pdf.NewExtractor(buf)
					extracted, err := ExtractEncryptedPayload(x, obj)
					if err != nil {
						t.Fatal(err)
					}

					// The extracted object should not have SingleUse set
					expected := original
					expected.SingleUse = false

					if diff := cmp.Diff(expected, *extracted); diff != "" {
						t.Errorf("round trip failed (-want +got):\n%s", diff)
					}
				})
			}
		})
	}
}

func TestEncryptedPayloadErrors(t *testing.T) {
	t.Run("missing subtype", func(t *testing.T) {
		ep := &EncryptedPayload{
			// Missing Subtype
			Version: "1.0",
		}

		buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
		rm := pdf.NewResourceManager(buf)

		_, _, err := ep.Embed(rm)
		if err == nil {
			t.Error("expected error for missing Subtype")
		}
	})

	t.Run("version requirement", func(t *testing.T) {
		ep := &EncryptedPayload{
			FilterName: "TestCrypto",
		}

		buf, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
		rm := pdf.NewResourceManager(buf)

		_, _, err := ep.Embed(rm)
		if err == nil {
			t.Error("expected version error for PDF 1.7")
		}
	})

	t.Run("malformed dict", func(t *testing.T) {
		buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
		x := pdf.NewExtractor(buf)

		// Create malformed dictionary (missing required Subtype)
		dict := pdf.Dict{
			"Type":    pdf.Name("EncryptedPayload"),
			"Version": pdf.TextString("1.0"),
			// Missing Subtype
		}

		_, err := ExtractEncryptedPayload(x, dict)
		if err == nil {
			t.Error("expected error for missing Subtype")
		}
	})
}

func TestEncryptedPayloadOptionalType(t *testing.T) {
	ep := &EncryptedPayload{
		FilterName: "TestCrypto",
		SingleUse:  true,
	}

	t.Run("type field handling", func(t *testing.T) {
		buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
		rm := pdf.NewResourceManager(buf)

		obj, _, err := ep.Embed(rm)
		if err != nil {
			t.Fatal(err)
		}

		dict, ok := obj.(pdf.Dict)
		if !ok {
			t.Fatal("expected dictionary")
		}

		// The Type field handling depends on PDF writer options
		// For now, just verify the dictionary is created correctly
		if dict["Subtype"] != pdf.Name("TestCrypto") {
			t.Error("missing or incorrect Subtype field")
		}
	})
}
