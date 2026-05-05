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
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestEncryptionString covers the documented String() formats:
// "None" for nil, "RC4 (N-bit)" for RC4, "<cipher>-N" for non-RC4
// ciphers, and "Unknown" when the cipher is unset.
func TestEncryptionString(t *testing.T) {
	cases := []struct {
		name string
		e    *pdf.Encryption
		want string
	}{
		{"Nil", nil, "None"},
		{"RC4_40", &pdf.Encryption{Cipher: "RC4", KeyLength: 40}, "RC4 (40-bit)"},
		{"RC4_128", &pdf.Encryption{Cipher: "RC4", KeyLength: 128}, "RC4 (128-bit)"},
		{"AES_128", &pdf.Encryption{Cipher: "AES", KeyLength: 128}, "AES-128"},
		{"AES_256", &pdf.Encryption{Cipher: "AES", KeyLength: 256}, "AES-256"},
		{"Unknown", &pdf.Encryption{}, "Unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.e.String(); got != c.want {
				t.Errorf("String() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestEncryptionMetaInfo verifies that a writer's encryption choice is
// reported through MetaInfo.Encryption and recovered by the reader at
// each PDF version that selects a different cipher.  This exercises the
// public observable: callers ask MetaInfo for the encryption summary
// and expect it to match what the writer was configured for.
func TestEncryptionMetaInfo(t *testing.T) {
	cases := []struct {
		name    string
		version pdf.Version
		want    pdf.Encryption
	}{
		{"V1_3_RC4_40", pdf.V1_3, pdf.Encryption{Cipher: "RC4", KeyLength: 40}},
		{"V1_5_RC4_128", pdf.V1_5, pdf.Encryption{Cipher: "RC4", KeyLength: 128}},
		{"V1_7_AES_128", pdf.V1_7, pdf.Encryption{Cipher: "AES", KeyLength: 128}},
		{"V2_0_AES_256", pdf.V2_0, pdf.Encryption{Cipher: "AES", KeyLength: 256}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opt := &pdf.WriterOptions{UserPassword: "u"}
			w, mf := memfile.NewPDFWriter(c.version, opt)

			// writer-side observation
			if got := w.GetMeta().Encryption; got == nil || *got != c.want {
				t.Errorf("writer Encryption = %+v, want %+v", got, c.want)
			}

			if err := memfile.AddBlankPage(w); err != nil {
				t.Fatalf("AddBlankPage: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}

			// reader-side observation
			r, err := pdf.NewReader(bytes.NewReader(mf.Data), int64(len(mf.Data)),
				&pdf.ReaderOptions{Password: "u"})
			if err != nil {
				t.Fatalf("NewReader: %v", err)
			}
			if got := r.GetMeta().Encryption; got == nil || *got != c.want {
				t.Errorf("reader Encryption = %+v, want %+v", got, c.want)
			}
		})
	}
}

// TestEncryptionMetaInfoUnencrypted verifies that the encryption
// summary is nil for unencrypted documents on both writer and reader
// sides, so callers can rely on a nil check to mean "no encryption".
func TestEncryptionMetaInfoUnencrypted(t *testing.T) {
	w, mf := memfile.NewPDFWriter(pdf.V2_0, nil)

	if got := w.GetMeta().Encryption; got != nil {
		t.Errorf("writer Encryption: got %+v, want nil", got)
	}

	if err := memfile.AddBlankPage(w); err != nil {
		t.Fatalf("AddBlankPage: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := pdf.NewReader(bytes.NewReader(mf.Data), int64(len(mf.Data)), nil)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	if got := r.GetMeta().Encryption; got != nil {
		t.Errorf("reader Encryption: got %+v, want nil", got)
	}
}
