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

package image

import (
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/opaque"
)

// streamData is the ImageData implementation for images read from a
// PDF file.  Both decoding (for Pixels) and verbatim cross-file
// re-emission (for WriteStream) are delegated to [opaque.Stream]; the
// nested-reference translation in /DecodeParms (JBIG2Globals etc.) is
// handled correctly by [pdf.Copier] under the hood.
type streamData struct {
	inner *opaque.Stream
}

// NewStreamData returns an [graphics.ImageData] that lazily reads from
// the given PDF stream.  On [graphics.ImageData.WriteStream], the
// stream is copied with its original encoding.  On
// [graphics.ImageData.Pixels], the stream is fully decoded.
func NewStreamData(x *pdf.Extractor, stream *pdf.Stream) graphics.ImageData {
	return &streamData{inner: opaque.ExtractStream(x, stream)}
}

// Pixels returns the fully decoded pixel data from the stream.
func (s *streamData) Pixels() ([]byte, error) {
	r, err := s.inner.Reader()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// WriteStream copies the stream to the destination PDF, preserving the
// original encoding.
func (s *streamData) WriteStream(rm *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	return s.inner.WriteAt(rm, ref, dict)
}

var _ graphics.ImageData = (*streamData)(nil)
