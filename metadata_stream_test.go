// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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

	pdfData, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(pdfData)
	ref, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm close: %v", err)
	}

	extracted, err := pdf.ExtractMetadataStream(pdf.NewCursor(pdfData), ref, false)
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

	pdfData, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(pdfData)
	ref, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm close: %v", err)
	}

	extracted, err := pdf.ExtractMetadataStream(pdf.NewCursor(pdfData), ref, false)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if extracted.Data.PadToLength != padTo {
		t.Errorf("PadToLength on read: got %d, want %d", extracted.Data.PadToLength, padTo)
	}

	stream, err := pdf.NewCursor(pdfData).Stream(ref)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	body, err := pdf.DecodeStream(pdfData, nil, stream)
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

	pdfData, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(pdfData)
	ref, err := rm.Embed(&pdf.MetadataStream{Data: packet})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm close: %v", err)
	}

	stream, err := pdf.NewCursor(pdfData).Stream(ref)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	body, err := pdf.DecodeStream(pdfData, nil, stream)
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
	w, mf := memfile.NewPDFWriter(t, pdf.V2_0, opt)
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := pdf.NewReader(bytes.NewReader(mf.Data), int64(len(mf.Data)),
		&pdf.ReaderOptions{Password: "u"})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	// /Encrypt /EncryptMetadata must be false
	encDict, _ := pdf.NewCursor(r).Dict(r.GetMeta().Trailer["Encrypt"])
	if v, ok := encDict["EncryptMetadata"].(pdf.Boolean); !ok || bool(v) {
		t.Errorf("/EncryptMetadata: got %v, want Boolean(false)", encDict["EncryptMetadata"])
	}

	// catalog metadata stream has no /Filter /Crypt entry
	catalogDict, _ := pdf.NewCursor(r).Dict(r.GetMeta().Trailer["Root"])
	metaRef, _ := catalogDict["Metadata"].(pdf.Reference)
	if metaRef == 0 {
		t.Fatal("catalog has no /Metadata entry")
	}
	stream, err := pdf.NewCursor(r).Stream(metaRef)
	if err != nil {
		t.Fatalf("GetStream metadata: %v", err)
	}
	// no filter at all on the catalog metadata stream â the encrypt-dict
	// flag exempts it from encryption, and unpadded plaintext metadata is
	// stored raw so external scanners can find the <?xpacket markers
	filters, err := pdf.GetFilters(r, nil, stream.Dict)
	if err != nil {
		t.Fatalf("GetFilters: %v", err)
	}
	if len(filters) != 0 {
		t.Errorf("catalog metadata has unexpected filters: %v", filters)
	}

	// raw on-disk bytes must contain the XMP packet markers
	body, err := pdf.DecodeStream(r, nil, stream)
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
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, opt)
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
	w, mf := memfile.NewPDFWriter(t, pdf.V2_0, opt)
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
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// raw file bytes must NOT contain the XMP packet markers â the stream
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
	got, err := pdf.ExtractMetadataStream(pdf.NewCursor(r), ref, false)
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
	w, mf := memfile.NewPDFWriter(t, pdf.V2_0, opt)
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
	stream, err := pdf.NewCursor(r).Stream(ref)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	filters, err := pdf.GetFilters(r, nil, stream.Dict)
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
		w, mf := memfile.NewPDFWriter(t, pdf.V2_0, opt)
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
		w, mf := memfile.NewPDFWriter(t, pdf.V2_0, opt)
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
		if err := w.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		r, err := pdf.NewReader(bytes.NewReader(mf.Data), int64(len(mf.Data)),
			&pdf.ReaderOptions{Password: "u"})
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		x := pdf.NewExtractor(r)
		md, err := pdf.ExtractMetadataStream(pdf.CursorAt(x, nil), ref, false)
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
		w, mf := memfile.NewPDFWriter(t, pdf.V2_0, opt)
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

	pdfData, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
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

			pdfData, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
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
				pdf.NewCursor(pdfData), ref, false)
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
		w, _ := memfile.NewPDFWriter(t, pdf.V1_4, opt)
		rm := pdf.NewResourceManager(w)
		_, err := rm.Embed(&pdf.MetadataStream{
			Data:      packet,
			Plaintext: true,
		})
		verErr, ok := errors.AsType[*pdf.VersionError](err)
		if !ok {
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
		verErr, ok := errors.AsType[*pdf.VersionError](err)
		if !ok {
			t.Fatalf("NewWriter: got %v, want *VersionError", err)
		}
		if verErr.Earliest != pdf.V1_6 {
			t.Errorf("VersionError.Earliest: got %v, want V1_6", verErr.Earliest)
		}
	})
}

// TestMetadataPaddedEncryptedStream checks that a padded XMP packet in an
// encrypted, unfiltered stream can be read and written back.  The bytes on
// disk are ciphertext, so the padding is not usable for in-place editing.
func TestMetadataPaddedEncryptedStream(t *testing.T) {
	packet := xmp.NewPacket()
	dc := &xmp.DublinCore{}
	dc.Title.Set(language.Und, "Encrypted Padded")
	if err := packet.Set(dc); err != nil {
		t.Fatalf("set: %v", err)
	}
	packet.PadToLength = 2048

	w1, mf := memfile.NewPDFWriter(t, pdf.V2_0, &pdf.WriterOptions{UserPassword: "u"})
	ref := w1.Alloc()
	body, err := w1.OpenStream(ref, pdf.Dict{
		"Type":    pdf.Name("Metadata"),
		"Subtype": pdf.Name("XML"),
	})
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	if err := packet.Write(body, nil); err != nil {
		t.Fatalf("write packet: %v", err)
	}
	if err := body.Close(); err != nil {
		t.Fatalf("close stream: %v", err)
	}

	if err := w1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	r, err := pdf.NewReader(bytes.NewReader(mf.Data), int64(len(mf.Data)),
		&pdf.ReaderOptions{Password: "u"})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	m1, err := pdf.ExtractMetadataStream(pdf.NewCursor(r), ref, false)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	w2, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w2)
	ref2, err := rm.Embed(m1)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm close: %v", err)
	}

	m2, err := pdf.ExtractMetadataStream(pdf.NewCursor(w2), ref2, false)
	if err != nil {
		t.Fatalf("extract after write: %v", err)
	}
	if !m1.Equal(m2) || m1.Plaintext != m2.Plaintext {
		t.Error("round trip failed")
	}
}

// TestMetadataPaddedExemptCatalog checks that the padding of the catalog
// metadata stream survives when /EncryptMetadata is false: the stream is
// stored raw on disk, so the padding stays usable for in-place editing.
func TestMetadataPaddedExemptCatalog(t *testing.T) {
	const padTo = 2048

	packet := xmp.NewPacket()
	dc := &xmp.DublinCore{}
	dc.Title.Set(language.Und, "Exempt Padded")
	if err := packet.Set(dc); err != nil {
		t.Fatalf("set: %v", err)
	}
	packet.PadToLength = padTo

	opt := &pdf.WriterOptions{
		UserPassword:     "u",
		DocumentMetadata: &pdf.MetadataStream{Data: packet, Plaintext: true},
	}
	w1, mf := memfile.NewPDFWriter(t, pdf.V2_0, opt)
	if err := w1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	r1, err := pdf.NewReader(bytes.NewReader(mf.Data), int64(len(mf.Data)),
		&pdf.ReaderOptions{Password: "u"})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	m1 := r1.GetMeta().Catalog.Metadata
	if m1 == nil {
		t.Fatal("no catalog metadata")
	}
	if !m1.Plaintext || m1.Data.PadToLength != padTo {
		t.Errorf("read: got Plaintext=%t, PadToLength=%d, want true, %d",
			m1.Plaintext, m1.Data.PadToLength, padTo)
	}

	t.Run("Writer", func(t *testing.T) {
		catalog, err := pdf.NewCursor(r1).Dict(r1.GetMeta().Trailer["Root"])
		if err != nil {
			t.Fatalf("catalog: %v", err)
		}
		m, err := pdf.ExtractMetadataStream(pdf.NewCursor(w1), catalog["Metadata"], false)
		if err != nil {
			t.Fatalf("extract from writer: %v", err)
		}
		if !m.Equal(m1) || m.Data.PadToLength != padTo {
			t.Error("read from writer differs from read from file")
		}
	})

	t.Run("Updater", func(t *testing.T) {
		catalog, err := pdf.NewCursor(r1).Dict(r1.GetMeta().Trailer["Root"])
		if err != nil {
			t.Fatalf("catalog: %v", err)
		}
		u, err := pdf.NewUpdaterTo(bytes.NewReader(mf.Data), int64(len(mf.Data)),
			&bytes.Buffer{}, &pdf.UpdateOptions{Password: "u"})
		if err != nil {
			t.Fatalf("NewUpdaterTo: %v", err)
		}
		m, err := pdf.ExtractMetadataStream(pdf.NewCursor(u), catalog["Metadata"], false)
		if err != nil {
			t.Fatalf("extract from updater: %v", err)
		}
		if !m.Equal(m1) || m.Data.PadToLength != padTo {
			t.Error("read from updater differs from read from file")
		}
	})

	t.Run("RoundTrip", func(t *testing.T) {
		opt := &pdf.WriterOptions{UserPassword: "u", DocumentMetadata: m1}
		w2, mf2 := memfile.NewPDFWriter(t, pdf.V2_0, opt)
		if err := w2.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		r2, err := pdf.NewReader(bytes.NewReader(mf2.Data), int64(len(mf2.Data)),
			&pdf.ReaderOptions{Password: "u"})
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		m2 := r2.GetMeta().Catalog.Metadata
		if !m1.Equal(m2) || m2.Plaintext != m1.Plaintext ||
			m2.Data.PadToLength != m1.Data.PadToLength {
			t.Error("round trip failed")
		}
	})
}

// TestMetadataCryptIdentityPlaintext checks that a stream whose only filter
// is /Crypt /Identity reads as plaintext, with its padding kept, where
// document encryption does not apply.
func TestMetadataCryptIdentityPlaintext(t *testing.T) {
	const padTo = 2048
	newPacket := func(t *testing.T, title string) *xmp.Packet {
		t.Helper()
		packet := xmp.NewPacket()
		dc := &xmp.DublinCore{}
		dc.Title.Set(language.Und, title)
		if err := packet.Set(dc); err != nil {
			t.Fatalf("set: %v", err)
		}
		packet.PadToLength = padTo
		return packet
	}
	writeStream := func(t *testing.T, w *pdf.Writer, ref pdf.Reference, packet *xmp.Packet) {
		t.Helper()
		body, err := w.OpenStream(ref, pdf.Dict{
			"Type":    pdf.Name("Metadata"),
			"Subtype": pdf.Name("XML"),
		}, pdf.FilterCryptIdentity{})
		if err != nil {
			t.Fatalf("OpenStream: %v", err)
		}
		if err := packet.Write(body, nil); err != nil {
			t.Fatalf("write packet: %v", err)
		}
		if err := body.Close(); err != nil {
			t.Fatalf("close stream: %v", err)
		}
	}
	check := func(t *testing.T, m1, m2 *pdf.MetadataStream) {
		t.Helper()
		if !m1.Plaintext || m1.Data.PadToLength != padTo {
			t.Errorf("read: got Plaintext=%t, PadToLength=%d, want true, %d",
				m1.Plaintext, m1.Data.PadToLength, padTo)
		}
		if !m1.Equal(m2) || m1.Plaintext != m2.Plaintext ||
			m1.Data.PadToLength != m2.Data.PadToLength {
			t.Error("round trip failed")
		}
	}

	t.Run("Unencrypted", func(t *testing.T) {
		w1, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		ref := w1.Alloc()
		writeStream(t, w1, ref, newPacket(t, "unencrypted"))
		m1, err := pdf.ExtractMetadataStream(pdf.NewCursor(w1), ref, false)
		if err != nil {
			t.Fatalf("extract: %v", err)
		}

		w2, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		rm := pdf.NewResourceManager(w2)
		ref2, err := rm.Embed(m1)
		if err != nil {
			t.Fatalf("embed: %v", err)
		}
		if err := rm.Close(); err != nil {
			t.Fatalf("rm close: %v", err)
		}
		m2, err := pdf.ExtractMetadataStream(pdf.NewCursor(w2), ref2, false)
		if err != nil {
			t.Fatalf("extract after write: %v", err)
		}
		check(t, m1, m2)
	})

	t.Run("ExemptCatalog", func(t *testing.T) {
		opt := &pdf.WriterOptions{
			UserPassword:     "u",
			DocumentMetadata: &pdf.MetadataStream{Data: newPacket(t, "base"), Plaintext: true},
		}
		w0, f := memfile.NewPDFWriter(t, pdf.V2_0, opt)
		if err := w0.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		ropt := &pdf.ReaderOptions{Password: "u"}
		r0, err := pdf.NewReader(bytes.NewReader(f.Data), int64(len(f.Data)), ropt)
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		catalog, err := pdf.NewCursor(r0).Dict(r0.GetMeta().Trailer["Root"])
		if err != nil {
			t.Fatalf("catalog: %v", err)
		}
		mref := catalog["Metadata"].(pdf.Reference)

		u, err := pdf.NewUpdater(f, int64(len(f.Data)), &pdf.UpdateOptions{Password: "u"})
		if err != nil {
			t.Fatalf("NewUpdater: %v", err)
		}
		writeStream(t, u, mref, newPacket(t, "replaced"))
		if err := u.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		r1, err := pdf.NewReader(bytes.NewReader(f.Data), int64(len(f.Data)), ropt)
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		m1 := r1.GetMeta().Catalog.Metadata

		w2, f2 := memfile.NewPDFWriter(t, pdf.V2_0,
			&pdf.WriterOptions{UserPassword: "u", DocumentMetadata: m1})
		if err := w2.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		r2, err := pdf.NewReader(bytes.NewReader(f2.Data), int64(len(f2.Data)), ropt)
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		check(t, m1, r2.GetMeta().Catalog.Metadata)
	})
}
