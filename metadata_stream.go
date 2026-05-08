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
// so callers can use [Optional] / [ExtractorGetOptional] to swallow
// them.
func ExtractMetadataStream(x *Extractor, path *CycleCheck, ref Object, _ bool) (*MetadataStream, error) {
	if ref == nil {
		return nil, &MalformedFileError{
			Err: errors.New("missing metadata stream"),
		}
	}

	stream, err := x.GetStream(path, ref)
	if err != nil {
		return nil, err
	}
	if stream == nil {
		return nil, nil
	}

	body, err := DecodeStream(x.R, path, stream, 0)
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

	// inspect the on-disk filter chain so we can preserve plaintext
	// semantics on rewrite and reset PadToLength when the source was
	// Flate-wrapped (the on-disk length the xmp parser observed is the
	// decoded length, not the stored length, so it's not a meaningful
	// pad target on rewrite).
	plaintext := false
	if filters, ferr := GetFilters(x.R, path, stream.Dict); ferr == nil {
		// the bytes on disk are raw XMP iff:
		//   - in an unencrypted file: there are no filters at all
		//   - in an encrypted file: the chain is exactly /Crypt /Identity,
		//     which bypasses document encryption without transforming
		//     the data; any additional filter (e.g. /FlateDecode) means
		//     the on-disk bytes are not raw XMP.
		// the catalog /EncryptMetadata=false case is handled by the
		// reader after extraction (it sets Plaintext on the typed value).
		if x.R.GetMeta().Encryption == nil {
			plaintext = len(filters) == 0
		} else if len(filters) == 1 {
			_, plaintext = filters[0].(FilterCryptIdentity)
		}
		if packet.PadToLength > 0 {
			for _, f := range filters {
				if _, ok := f.(FilterFlate); ok {
					packet.PadToLength = 0
					break
				}
			}
		}
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

	// refIsPlaintext is set by the writer for the catalog metadata stream
	// when DocumentMetadata.Plaintext is true; in that case OpenStream
	// skips encryption and no per-stream filter is needed.  Other
	// plaintext streams need /Filter /Crypt /Identity to bypass document
	// encryption.
	var filters []Filter
	if s.Plaintext && w.isEncrypted() && !w.refIsPlaintext[ref] {
		if err := CheckVersion(w, "plaintext metadata stream", V1_5); err != nil {
			return nil, err
		}
		filters = append(filters, FilterCryptIdentity{})
	}
	if !s.Plaintext && !w.refIsPlaintext[ref] {
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
