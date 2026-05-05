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
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/xmp"
)

func TestMetadataRoundTrip(t *testing.T) {
	packet := xmp.NewPacket()
	dc := &xmp.DublinCore{}
	dc.Title.Set(language.Und, "Test Document")
	dc.Creator.Append(xmp.NewProperName("Test Author"))
	if err := packet.Set(dc); err != nil {
		t.Fatalf("failed to set properties: %v", err)
	}

	original := &pdf.MetadataStream{Data: packet}

	pdfData, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(pdfData)
	ref, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm close: %v", err)
	}

	extracted, err := pdf.ExtractMetadataStream(pdf.NewExtractor(pdfData), nil, ref, false)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	var originalDC, extractedDC xmp.DublinCore
	if err := original.Data.Get(&originalDC); err != nil {
		t.Fatalf("get original: %v", err)
	}
	if err := extracted.Data.Get(&extractedDC); err != nil {
		t.Fatalf("get extracted: %v", err)
	}
	if diff := cmp.Diff(extractedDC, originalDC); diff != "" {
		t.Errorf("round trip failed (-got +want):\n%s", diff)
	}
}

func TestMetadataRoundTripPadded(t *testing.T) {
	const padTo = 4096

	packet := xmp.NewPacket()
	dc := &xmp.DublinCore{}
	dc.Title.Set(language.Und, "Padded Test Document")
	dc.Creator.Append(xmp.NewProperName("Test Author"))
	if err := packet.Set(dc); err != nil {
		t.Fatalf("set: %v", err)
	}

	packet.PadToLength = padTo
	original := &pdf.MetadataStream{Data: packet, Plaintext: true}

	pdfData, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(pdfData)
	ref, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm close: %v", err)
	}

	extracted, err := pdf.ExtractMetadataStream(pdf.NewExtractor(pdfData), nil, ref, false)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if extracted.Data.PadToLength != padTo {
		t.Errorf("PadToLength on read: got %d, want %d", extracted.Data.PadToLength, padTo)
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
	if len(raw) != padTo {
		t.Errorf("on-disk length: got %d, want %d", len(raw), padTo)
	}
	if !bytes.Contains(raw, []byte(`<?xpacket end="w"?>`)) {
		t.Errorf("trailer is not the writable form")
	}
}

func TestMetadataUnpaddedTrailer(t *testing.T) {
	packet := xmp.NewPacket()
	dc := &xmp.DublinCore{}
	dc.Title.Set(language.Und, "Read-only")
	if err := packet.Set(dc); err != nil {
		t.Fatalf("set: %v", err)
	}

	pdfData, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(pdfData)
	ref, err := rm.Embed(&pdf.MetadataStream{Data: packet})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm close: %v", err)
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
		t.Errorf("trailer is not the read-only form")
	}
}

// TestMetadataCatalogPlaintext verifies that on an encrypted V2_0 doc
// with WriterOptions.CatalogMetadata.Plaintext=true, the catalog
// metadata stream has no /Filter /Crypt entry, the on-disk bytes are
// plaintext XMP, /Encrypt has /EncryptMetadata false, and
// round-tripping through the reader recovers the same XMP packet.
func TestMetadataCatalogPlaintext(t *testing.T) {
	packet := xmp.NewPacket()
	dc := &xmp.DublinCore{}
	dc.Title.Set(language.Und, "Encrypted Test")
	if err := packet.Set(dc); err != nil {
		t.Fatalf("set: %v", err)
	}

	opt := &pdf.WriterOptions{
		UserPassword:  "u",
		OwnerPassword: "o",
		DocumentMetadata: &pdf.MetadataStream{
			Data:      packet,
			Plaintext: true,
		},
	}
	w, mf := memfile.NewPDFWriter(pdf.V2_0, opt)
	if err := memfile.AddBlankPage(w); err != nil {
		t.Fatalf("AddBlankPage: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := pdf.NewReader(bytes.NewReader(mf.Data), int64(len(mf.Data)),
		&pdf.ReaderOptions{Password: "u"})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	// /Encrypt /EncryptMetadata must be false
	encDict, _ := pdf.GetDict(r, r.GetMeta().Trailer["Encrypt"])
	if v, ok := encDict["EncryptMetadata"].(pdf.Boolean); !ok || bool(v) {
		t.Errorf("/EncryptMetadata: got %v, want Boolean(false)", encDict["EncryptMetadata"])
	}

	// catalog metadata stream has no /Filter /Crypt entry
	catalogDict, _ := pdf.GetDict(r, r.GetMeta().Trailer["Root"])
	metaRef, _ := catalogDict["Metadata"].(pdf.Reference)
	if metaRef == 0 {
		t.Fatal("catalog has no /Metadata entry")
	}
	stream, err := pdf.GetStream(r, metaRef)
	if err != nil {
		t.Fatalf("GetStream metadata: %v", err)
	}
	// no filter at all on the catalog metadata stream — the encrypt-dict
	// flag exempts it from encryption, and unpadded plaintext metadata is
	// stored raw so external scanners can find the <?xpacket markers
	filters, err := pdf.GetFilters(r, stream.Dict)
	if err != nil {
		t.Fatalf("GetFilters: %v", err)
	}
	if len(filters) != 0 {
		t.Errorf("catalog metadata has unexpected filters: %v", filters)
	}

	// raw on-disk bytes must contain the XMP packet markers
	body, err := pdf.DecodeStream(r, stream, 0)
	if err != nil {
		t.Fatalf("DecodeStream: %v", err)
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if !bytes.Contains(raw, []byte(`<?xpacket begin=`)) {
		t.Errorf("on-disk metadata is not raw XMP (no <?xpacket marker)")
	}

	// reader populates the typed Metadata field
	if r.GetMeta().Catalog.Metadata == nil {
		t.Fatal("Reader.GetMeta().Catalog.Metadata is nil")
	}
	var rtDC, origDC xmp.DublinCore
	if err := r.GetMeta().Catalog.Metadata.Data.Get(&rtDC); err != nil {
		t.Fatalf("Get rtDC: %v", err)
	}
	if err := packet.Get(&origDC); err != nil {
		t.Fatalf("Get origDC: %v", err)
	}
	if diff := cmp.Diff(rtDC, origDC); diff != "" {
		t.Errorf("round trip XMP (-got +want):\n%s", diff)
	}
}

// TestMetadataPaddedCatalogRequiresPlaintext verifies that Writer.Close
// rejects replacing Catalog.Metadata after NewWriter.  The catalog
// metadata stream is committed eagerly during NewWriter (so the
// encryption key derivation can see the plaintext policy), and the
// pointer-identity check at Close catches any later substitution.
func TestMetadataPaddedCatalogRequiresPlaintext(t *testing.T) {
	packet := xmp.NewPacket()
	if err := packet.Set(&xmp.DublinCore{}); err != nil {
		t.Fatalf("set: %v", err)
	}

	opt := &pdf.WriterOptions{
		UserPassword: "u",
	}
	w, _ := memfile.NewPDFWriter(pdf.V2_0, opt)
	if err := memfile.AddBlankPage(w); err != nil {
		t.Fatalf("AddBlankPage: %v", err)
	}
	// no DocumentMetadata was declared up front, so Catalog.Metadata
	// was nil at NewWriter; replacing it here trips the pointer check
	packet.PadToLength = 1024
	w.GetMeta().Catalog.Metadata = &pdf.MetadataStream{
		Data:      packet,
		Plaintext: true,
	}
	if err := w.Close(); err == nil {
		t.Error("Writer.Close: expected error for catalog metadata not declared via WriterOptions.DocumentMetadata, got nil")
	}
}

// TestMetadataNonCatalogStreamEncrypted verifies that a non-catalog
// MetadataStream embedded via rm.Embed in an encrypted document
// is encrypted on disk by default.  /EncryptMetadata only governs the
// catalog metadata stream; non-catalog streams need MetadataStream.Plaintext
// to opt out of encryption.
func TestMetadataNonCatalogStreamEncrypted(t *testing.T) {
	packet := xmp.NewPacket()
	dc := &xmp.DublinCore{}
	dc.Title.Set(language.Und, "Non-catalog metadata")
	if err := packet.Set(dc); err != nil {
		t.Fatalf("set: %v", err)
	}

	opt := &pdf.WriterOptions{
		UserPassword: "u",
	}
	w, mf := memfile.NewPDFWriter(pdf.V2_0, opt)
	rm := pdf.NewResourceManager(w)
	embedded, err := rm.Embed(&pdf.MetadataStream{Data: packet})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	ref, ok := embedded.(pdf.Reference)
	if !ok {
		t.Fatalf("embed returned non-reference: %T", embedded)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm close: %v", err)
	}
	if err := memfile.AddBlankPage(w); err != nil {
		t.Fatalf("AddBlankPage: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// raw file bytes must NOT contain the XMP packet markers — the stream
	// is encrypted on disk because /EncryptMetadata only exempts the
	// catalog metadata stream
	if bytes.Contains(mf.Data, []byte(`<?xpacket begin=`)) {
		t.Error("non-catalog metadata stream is plaintext on disk; expected encrypted bytes")
	}

	// re-open and round-trip through the reader to confirm the stream is
	// readable with the password and decodes back to the same XMP
	r, err := pdf.NewReader(bytes.NewReader(mf.Data), int64(len(mf.Data)),
		&pdf.ReaderOptions{Password: "u"})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	got, err := pdf.ExtractMetadataStream(pdf.NewExtractor(r), nil, ref, false)
	if err != nil {
		t.Fatalf("ExtractMetadataStream: %v", err)
	}
	var rtDC, origDC xmp.DublinCore
	if err := got.Data.Get(&rtDC); err != nil {
		t.Fatalf("Get rtDC: %v", err)
	}
	if err := packet.Get(&origDC); err != nil {
		t.Fatalf("Get origDC: %v", err)
	}
	if diff := cmp.Diff(rtDC, origDC); diff != "" {
		t.Errorf("round trip XMP (-got +want):\n%s", diff)
	}
}

// TestMetadataNonCatalogPlaintext verifies that MetadataStream.Plaintext = true
// on a non-catalog stream produces raw XMP bytes on disk and a /Filter
// chain that begins with /Crypt /Identity inside an encrypted document.
func TestMetadataNonCatalogPlaintext(t *testing.T) {
	packet := xmp.NewPacket()
	dc := &xmp.DublinCore{}
	dc.Title.Set(language.Und, "Plaintext non-catalog metadata")
	if err := packet.Set(dc); err != nil {
		t.Fatalf("set: %v", err)
	}

	opt := &pdf.WriterOptions{
		UserPassword: "u",
	}
	w, mf := memfile.NewPDFWriter(pdf.V2_0, opt)
	rm := pdf.NewResourceManager(w)
	embedded, err := rm.Embed(&pdf.MetadataStream{
		Data:      packet,
		Plaintext: true,
	})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	ref, ok := embedded.(pdf.Reference)
	if !ok {
		t.Fatalf("embed returned non-reference: %T", embedded)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm close: %v", err)
	}
	if err := memfile.AddBlankPage(w); err != nil {
		t.Fatalf("AddBlankPage: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// raw on-disk bytes must contain the XMP packet markers
	if !bytes.Contains(mf.Data, []byte(`<?xpacket begin=`)) {
		t.Error("plaintext non-catalog metadata has no <?xpacket marker on disk")
	}

	r, err := pdf.NewReader(bytes.NewReader(mf.Data), int64(len(mf.Data)),
		&pdf.ReaderOptions{Password: "u"})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	stream, err := pdf.GetStream(r, ref)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	filters, err := pdf.GetFilters(r, stream.Dict)
	if err != nil {
		t.Fatalf("GetFilters: %v", err)
	}
	if len(filters) == 0 {
		t.Fatal("expected /Filter /Crypt on non-catalog plaintext stream, got none")
	}
	if _, ok := filters[0].(pdf.FilterCryptIdentity); !ok {
		t.Errorf("first filter: got %T, want pdf.FilterCryptIdentity", filters[0])
	}
}

// TestMetadataPlaintextRoundTrip verifies that the reader recovers the
// Plaintext flag from both the catalog-level /EncryptMetadata=false
// path and the per-stream /Filter /Crypt /Identity path, so the typed
// MetadataStream can drive an unchanged rewrite.
func TestMetadataPlaintextRoundTrip(t *testing.T) {
	t.Run("CatalogEncryptMetadataFalse", func(t *testing.T) {
		packet := xmp.NewPacket()
		if err := packet.Set(&xmp.DublinCore{}); err != nil {
			t.Fatalf("set: %v", err)
		}

		opt := &pdf.WriterOptions{
			UserPassword: "u",
			DocumentMetadata: &pdf.MetadataStream{
				Data:      packet,
				Plaintext: true,
			},
		}
		w, mf := memfile.NewPDFWriter(pdf.V2_0, opt)
		if err := memfile.AddBlankPage(w); err != nil {
			t.Fatalf("AddBlankPage: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		r, err := pdf.NewReader(bytes.NewReader(mf.Data), int64(len(mf.Data)),
			&pdf.ReaderOptions{Password: "u"})
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		md := r.GetMeta().Catalog.Metadata
		if md == nil {
			t.Fatal("Catalog.Metadata is nil")
		}
		if !md.Plaintext {
			t.Error("Catalog.Metadata.Plaintext: got false, want true")
		}
	})

	t.Run("StreamCryptIdentity", func(t *testing.T) {
		packet := xmp.NewPacket()
		if err := packet.Set(&xmp.DublinCore{}); err != nil {
			t.Fatalf("set: %v", err)
		}

		opt := &pdf.WriterOptions{UserPassword: "u"}
		w, mf := memfile.NewPDFWriter(pdf.V2_0, opt)
		rm := pdf.NewResourceManager(w)
		embedded, err := rm.Embed(&pdf.MetadataStream{
			Data:      packet,
			Plaintext: true,
		})
		if err != nil {
			t.Fatalf("embed: %v", err)
		}
		ref := embedded.(pdf.Reference)
		if err := rm.Close(); err != nil {
			t.Fatalf("rm close: %v", err)
		}
		if err := memfile.AddBlankPage(w); err != nil {
			t.Fatalf("AddBlankPage: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		r, err := pdf.NewReader(bytes.NewReader(mf.Data), int64(len(mf.Data)),
			&pdf.ReaderOptions{Password: "u"})
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		x := pdf.NewExtractor(r)
		md, err := pdf.ExtractMetadataStream(x, nil, ref, false)
		if err != nil {
			t.Fatalf("ExtractMetadataStream: %v", err)
		}
		if !md.Plaintext {
			t.Error("non-catalog Plaintext: got false, want true")
		}
	})

	t.Run("EncryptedDefaultPlaintextFalse", func(t *testing.T) {
		packet := xmp.NewPacket()
		if err := packet.Set(&xmp.DublinCore{}); err != nil {
			t.Fatalf("set: %v", err)
		}

		opt := &pdf.WriterOptions{
			UserPassword:     "u",
			DocumentMetadata: &pdf.MetadataStream{Data: packet},
		}
		w, mf := memfile.NewPDFWriter(pdf.V2_0, opt)
		if err := memfile.AddBlankPage(w); err != nil {
			t.Fatalf("AddBlankPage: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		r, err := pdf.NewReader(bytes.NewReader(mf.Data), int64(len(mf.Data)),
			&pdf.ReaderOptions{Password: "u"})
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		md := r.GetMeta().Catalog.Metadata
		if md == nil {
			t.Fatal("Catalog.Metadata is nil")
		}
		if md.Plaintext {
			t.Error("Catalog.Metadata.Plaintext: got true, want false (default-encrypted)")
		}
	})
}

// TestMetadataPadToLengthRequiresPlaintext verifies that a packet with
// PadToLength > 0 attached to a MetadataStream with Plaintext=false is
// rejected at embed time.  In-place editing requires the stored bytes
// to be raw XMP, so the two flags must agree.
func TestMetadataPadToLengthRequiresPlaintext(t *testing.T) {
	packet := xmp.NewPacket()
	if err := packet.Set(&xmp.DublinCore{}); err != nil {
		t.Fatalf("set: %v", err)
	}
	packet.PadToLength = 1024

	pdfData, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(pdfData)
	_, err := rm.Embed(&pdf.MetadataStream{Data: packet})
	if err == nil {
		t.Fatal("rm.Embed: expected error for PadToLength without Plaintext, got nil")
	}
}

// TestMetadataPlaintextUnencryptedRoundTrip verifies that on an
// unencrypted document the read side recovers Plaintext from the
// absence of stream filters: writing Plaintext=true emits no Flate
// wrapper, and reading back yields Plaintext=true again.  The default
// (Plaintext=false) round-trips as false because the writer adds
// FilterFlate and the reader sees a non-empty filter chain.
func TestMetadataPlaintextUnencryptedRoundTrip(t *testing.T) {
	for _, plaintext := range []bool{false, true} {
		t.Run(fmt.Sprintf("Plaintext=%v", plaintext), func(t *testing.T) {
			packet := xmp.NewPacket()
			if err := packet.Set(&xmp.DublinCore{}); err != nil {
				t.Fatalf("set: %v", err)
			}

			pdfData, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(pdfData)
			ref, err := rm.Embed(&pdf.MetadataStream{
				Data:      packet,
				Plaintext: plaintext,
			})
			if err != nil {
				t.Fatalf("embed: %v", err)
			}
			if err := rm.Close(); err != nil {
				t.Fatalf("rm close: %v", err)
			}

			extracted, err := pdf.ExtractMetadataStream(
				pdf.NewExtractor(pdfData), nil, ref, false)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if extracted.Plaintext != plaintext {
				t.Errorf("Plaintext on read: got %v, want %v",
					extracted.Plaintext, plaintext)
			}
		})
	}
}

// TestMetadataPlaintextRequiresVersion verifies the documented version
// floors for MetadataStream.Plaintext in an encrypted document:
// /Filter /Crypt /Identity (per-stream) requires PDF 1.5, and
// /EncryptMetadata=false (catalog) requires PDF 1.6.  Both report the
// failure as *VersionError so callers can branch on the error type.
func TestMetadataPlaintextRequiresVersion(t *testing.T) {
	t.Run("NonCatalogBelowV1_5", func(t *testing.T) {
		packet := xmp.NewPacket()
		if err := packet.Set(&xmp.DublinCore{}); err != nil {
			t.Fatalf("set: %v", err)
		}

		opt := &pdf.WriterOptions{UserPassword: "u"}
		w, _ := memfile.NewPDFWriter(pdf.V1_4, opt)
		rm := pdf.NewResourceManager(w)
		_, err := rm.Embed(&pdf.MetadataStream{
			Data:      packet,
			Plaintext: true,
		})
		var verErr *pdf.VersionError
		if !errors.As(err, &verErr) {
			t.Fatalf("rm.Embed: got %v, want *VersionError", err)
		}
		if verErr.Earliest != pdf.V1_5 {
			t.Errorf("VersionError.Earliest: got %v, want V1_5", verErr.Earliest)
		}
	})

	t.Run("CatalogBelowV1_6", func(t *testing.T) {
		packet := xmp.NewPacket()
		if err := packet.Set(&xmp.DublinCore{}); err != nil {
			t.Fatalf("set: %v", err)
		}

		opt := &pdf.WriterOptions{
			UserPassword: "u",
			DocumentMetadata: &pdf.MetadataStream{
				Data:      packet,
				Plaintext: true,
			},
		}
		_, err := pdf.NewWriter(io.Discard, pdf.V1_5, opt)
		var verErr *pdf.VersionError
		if !errors.As(err, &verErr) {
			t.Fatalf("NewWriter: got %v, want *VersionError", err)
		}
		if verErr.Earliest != pdf.V1_6 {
			t.Errorf("VersionError.Earliest: got %v, want V1_6", verErr.Earliest)
		}
	})
}
