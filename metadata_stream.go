// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pdf

import (
	"errors"

	"seehuhn.de/go/xmp"
)

// PDF 2.0 sections: 14.3

// MetadataStream represents an XMP metadata stream.
//
// The metadata may either refer to a PDF document as a whole, or to
// individual objects within the document.  Used as the type of
// [Catalog.Metadata] for the document-level metadata stream and as an
// [Embedder] for metadata streams attached to other objects.
//
// Padding for in-place editing is controlled by [xmp.Packet.PadToLength]
// on the embedded packet; using padding requires Plaintext to be set on
// this wrapper.
type MetadataStream struct {
	// Data is the XMP packet carried by this stream.  It must be non-nil:
	// [ExtractMetadataStream] always returns a stream whose Data is set,
	// and callers constructing a MetadataStream directly must set Data
	// before passing the value to [ResourceManager.Embed] or assigning it
	// to [Catalog.Metadata].
	Data *xmp.Packet

	// Plaintext, if true, requests that the stream's bytes be written raw on
	// disk so external tools can locate them via byte-scanning.
	// This prevents compression and encryption of the metadata stream.
	//
	// If Data.PadToLength is non-zero, Plaintext must be true.
	Plaintext bool
}

// ExtractMetadataStream reads an XMP metadata stream from a PDF file.
//
// I/O errors from the underlying file are surfaced unchanged.  Errors
// caused by malformed XMP data are returned as [*MalformedFileError]
// so callers can use [Optional] / [Cursor.DecodeOptional] to swallow
// them.
func ExtractMetadataStream(c Cursor, ref Object, isDirect bool) (*MetadataStream, error) {
	if ref == nil {
		return nil, &MalformedFileError{
			Err: errors.New("missing metadata stream"),
		}
	}

	resolved, path, err := resolvePath(c.x.R, c.path, ref, true)
	if err != nil {
		return nil, err
	}
	stream, err := as[*Stream](resolved)
	if err != nil {
		return nil, err
	}
	if stream == nil {
		return nil, nil
	}

	body, err := c.StreamReader(stream)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	packet, err := xmp.Read(body)
	if err != nil {
		if errors.Is(err, xmp.ErrMalformed) {
			return nil, &MalformedFileError{Err: err}
		}
		return nil, err
	}

	// Inspect the on-disk filter chain so we can preserve plaintext
	// semantics on rewrite.  The xmp parser observed the decoded bytes;
	// unless these are also the bytes on disk, the packet length is not
	// a meaningful pad target and PadToLength is reset.
	// The stream is stored under the last reference followed, either
	// above or, when called via [Cursor.Decode], by the caller.
	encrypted := c.Getter().GetMeta().Encryption != nil
	_, isRef := ref.(Reference)
	if encrypted && (isRef || !isDirect) && path != nil {
		if g, ok := c.Getter().(exemptGetter); ok && g.isUnencrypted(path.Ref) {
			encrypted = false
		}
	}

	plaintext := false
	if filters, ferr := c.Filters(stream.Dict); ferr == nil {
		// The bytes on disk are raw XMP if there are no filters and
		// document encryption does not apply (unencrypted file, or the
		// catalog stream with /EncryptMetadata false), or if the chain is
		// exactly /Crypt /Identity, which neither transforms the data nor
		// applies encryption.
		// For the catalog stream with /EncryptMetadata false, the reader
		// sets Plaintext after extraction even when the bytes are
		// filtered, so that a rewrite keeps the policy.
		switch len(filters) {
		case 0:
			plaintext = !encrypted
		case 1:
			_, plaintext = filters[0].(FilterCryptIdentity)
		}
	}
	if !plaintext {
		packet.PadToLength = 0
	}

	return &MetadataStream{Data: packet, Plaintext: plaintext}, nil
}

// Embed adds the XMP metadata stream to the PDF file.
// This implements the [Embedder] interface.
func (s *MetadataStream) Embed(rm *EmbedHelper) (Native, error) {
	w := rm.Out()
	if err := CheckVersion(w, "XMP metadata stream", V1_4); err != nil {
		return nil, err
	}
	if s.Data.PadToLength > 0 && !s.Plaintext {
		return nil, errors.New("MetadataStream: PadToLength requires Plaintext")
	}
	ref := rm.AllocSelf()

	dict := Dict{
		"Type":    Name("Metadata"),
		"Subtype": Name("XML"),
	}

	// w.unencrypted holds the catalog metadata stream when
	// /EncryptMetadata is false; OpenStream then skips encryption and no
	// per-stream filter is needed.  Other plaintext streams need
	// /Filter /Crypt /Identity to bypass document encryption.
	var filters []Filter
	if s.Plaintext && w.isEncrypted() && !w.unencrypted[ref] {
		if err := CheckVersion(w, "plaintext metadata stream", V1_5); err != nil {
			return nil, err
		}
		filters = append(filters, FilterCryptIdentity{})
	}
	if !s.Plaintext && !w.unencrypted[ref] {
		filters = append(filters, FilterFlate{})
	}

	body, err := w.OpenStream(ref, dict, filters...)
	if err != nil {
		return nil, err
	}

	opts := &xmp.PacketOptions{
		Pretty: w.GetOptions().HasAny(OptPretty),
	}
	if err := s.Data.Write(body, opts); err != nil {
		return nil, err
	}
	if err := body.Close(); err != nil {
		return nil, err
	}
	return ref, nil
}

// Equal reports whether s and other represent the same XMP metadata.
// Serialization concerns — PadToLength on the inner packet, and the
// Plaintext flag on this wrapper — do not affect equality.
func (s *MetadataStream) Equal(other *MetadataStream) bool {
	if s == nil || other == nil {
		return s == other
	}
	return s.Data.Equal(other.Data)
}
