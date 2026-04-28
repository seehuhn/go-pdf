// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/xmp"
)

// PDF 2.0 sections: 14.3

// Stream represents an XMP metadata stream.
//
// The metadata may either refer to a PDF document as a whole, or to
// individual objects within the document.
type Stream struct {
	Data *xmp.Packet

	// PadToLength, if positive, pads the encoded XMP packet with whitespace
	// so the resulting PDF stream has exactly the given length in bytes.
	// The XMP trailer is then written as <?xpacket end="w"?>, signalling
	// that the packet may be edited in place inside the host PDF.
	//
	// When PadToLength is positive, the metadata stream is written
	// uncompressed so the on-disk length is predictable.  If document-level
	// encryption would otherwise apply to the stream, FilterCryptIdentity is
	// used to keep the bytes plaintext on disk.
	//
	// If the encoded packet does not fit in PadToLength bytes, Embed returns
	// xmp.ErrPacketTooLong.
	PadToLength int
}

func Extract(x *pdf.Extractor, path *pdf.CycleCheck, ref pdf.Object, _ bool) (*Stream, error) {
	if ref == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing metadata stream"),
		}
	}
	body, err := pdf.GetStreamReader(x.R, ref)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	packet, err := xmp.Read(body)
	if err != nil {
		return nil, err
	}

	return &Stream{Data: packet}, nil
}

// Embed adds the XMP metadata stream to the PDF file.
// This implements the [pdf.Embedder] interface.
func (s *Stream) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	w := rm.Out()
	if err := pdf.CheckVersion(w, "XMP metadata stream", pdf.V1_4); err != nil {
		return nil, err
	}
	ref := w.Alloc()

	dict := pdf.Dict{
		"Type":    pdf.Name("Metadata"),
		"Subtype": pdf.Name("XML"),
	}

	var filters []pdf.Filter
	info := rm.GetInfo()
	// per-stream Identity overrides the document encryption
	if info.DocumentEncrypted && (s.PadToLength > 0 || !info.MetadataEncrypted) {
		filters = append(filters, pdf.FilterCryptIdentity{})
	}
	if s.PadToLength == 0 {
		filters = append(filters, pdf.FilterFlate{})
	}

	body, err := w.OpenStream(ref, dict, filters...)
	if err != nil {
		return nil, err
	}

	opts := &xmp.PacketOptions{
		PadToLength: s.PadToLength,
		Pretty:      w.GetOptions().HasAny(pdf.OptPretty),
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
func (s *Stream) Equal(other *Stream) bool {
	if s == nil || other == nil {
		return s == other
	}

	return s.Data.Equal(other.Data)
}
