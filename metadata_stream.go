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
// on the embedded packet, not by this wrapper.
type MetadataStream struct {
	// Data is the XMP packet carried by this stream.  It must be non-nil:
	// [ExtractMetadataStream] always returns a stream whose Data is set,
	// and callers constructing a MetadataStream directly must set Data
	// before passing the value to [ResourceManager.Embed] or assigning it
	// to [Catalog.Metadata].
	Data *xmp.Packet
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

	body, err := DecodeStream(x.R, stream, 0)
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

	// Reset PadToLength when the source was Flate-wrapped — the on-disk
	// length the xmp parser observed is the decoded length, not the
	// stored length, so it's not a meaningful pad target on rewrite.
	if packet.PadToLength > 0 {
		if filters, ferr := GetFilters(x.R, stream.Dict); ferr == nil {
			for _, f := range filters {
				if _, ok := f.(FilterFlate); ok {
					packet.PadToLength = 0
					break
				}
			}
		}
	}

	return &MetadataStream{Data: packet}, nil
}

// Embed adds the XMP metadata stream to the PDF file.
// This implements the [Embedder] interface.
func (s *MetadataStream) Embed(rm *EmbedHelper) (Native, error) {
	w := rm.Out()
	if err := CheckVersion(w, "XMP metadata stream", V1_4); err != nil {
		return nil, err
	}
	ref := rm.AllocSelf()

	dict := Dict{
		"Type":    Name("Metadata"),
		"Subtype": Name("XML"),
	}

	var filters []Filter
	// non-catalog padded streams need an explicit Crypt /Identity to keep
	// their bytes plaintext on disk; the catalog-metadata path marks ref
	// as plaintext on the writer, in which case we skip the redundant filter
	if s.Data.PadToLength > 0 && w.isEncrypted() && !w.refIsPlaintext[ref] {
		filters = append(filters, FilterCryptIdentity{})
	}
	// Flate is skipped in two cases:
	//   - PadToLength > 0: compression would change the on-disk length and
	//     defeat the pad target.
	//   - the ref is marked plaintext: raw XMP keeps the <?xpacket markers
	//     visible to external tools that scan unencrypted metadata streams.
	if s.Data.PadToLength == 0 && !w.refIsPlaintext[ref] {
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
// PadToLength is a serialization concern (carried on the inner Packet)
// and does not affect equality.
func (s *MetadataStream) Equal(other *MetadataStream) bool {
	if s == nil || other == nil {
		return s == other
	}
	return s.Data.Equal(other.Data)
}
