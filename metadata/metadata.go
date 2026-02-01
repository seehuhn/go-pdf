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
}

func Extract(x *pdf.Extractor, ref pdf.Object) (*Stream, error) {
	if ref == nil {
		return nil, nil
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
	body, err := w.OpenStream(ref, dict, pdf.FilterFlate{})
	if err != nil {
		return nil, err
	}

	err = s.Data.Write(body, nil)
	if err != nil {
		return nil, err
	}

	err = body.Close()
	if err != nil {
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
