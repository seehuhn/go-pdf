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
	"maps"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/limits"
	"seehuhn.de/go/pdf/opaque"
)

// streamData is the ImageData implementation for images read from a
// PDF file.  Both decoding (for Pixels) and verbatim cross-file
// re-emission (for WriteStream) are delegated to [opaque.Stream]; the
// nested-reference translation in /DecodeParms (JBIG2Globals etc.) is
// handled correctly by [pdf.Copier] under the hood.
type streamData struct {
	inner    *opaque.Stream
	maxBytes int64 // per-image decoded-size cap

	// For JPXDecode images, jpx is the image stream with the filter chain
	// cut short before JPXDecode, read through x.  JPXDecode itself is
	// applied by [Dict.Load].
	jpx *pdf.Stream
	x   *pdf.Extractor
}

// Pixels returns the fully decoded pixel data from the stream.  The
// read is capped at the per-image budget computed from the dict's
// declared /Width, /Height, /BitsPerComponent and /ColorSpace, with
// any extra bytes the filter produces silently discarded.  Discarding
// is what viewers do when the filter (notably DCTDecode) emits more
// bytes than the dict declares, see viewer-tests/image/jpeg-mismatch;
// the short-read case is handled by [normalizeData] padding.
//
// For JPXDecode images, Pixels returns the JPEG 2000 data.
func (s *streamData) Pixels() ([]byte, error) {
	var r io.ReadCloser
	var err error
	if s.jpx != nil {
		r, err = pdf.DecodeStream(s.x.R, nil, s.jpx)
	} else {
		r, err = s.inner.Reader()
	}
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, s.maxBytes))
}

// IsJPX implements [graphics.ImageData].
func (s *streamData) IsJPX() bool {
	return s.jpx != nil
}

// WriteStream copies the stream to the destination PDF, preserving the
// original encoding.
func (s *streamData) WriteStream(rm *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	return s.inner.WriteAt(rm, ref, dict)
}

var _ graphics.ImageData = (*streamData)(nil)

// jpxFilterIndex returns the position of JPXDecode in the filter chain,
// or -1 if the chain does not use JPXDecode.
func jpxFilterIndex(filters []pdf.Filter) int {
	for i, f := range filters {
		if _, ok := f.(pdf.FilterJPX); ok {
			return i
		}
	}
	return -1
}

// newStreamData returns the image data of an image stream, where jpxIndex
// is the result of [jpxFilterIndex].
// maxBytes caps the decoded samples of non-JPX data; the JPEG 2000 data
// of a JPXDecode stream is capped at [limits.MaxImageBytes].
func newStreamData(c pdf.Cursor, stream *pdf.Stream, jpxIndex int, maxBytes int64) (*streamData, error) {
	data := &streamData{
		inner:    opaque.ExtractStream(c.Extractor(), stream),
		maxBytes: maxBytes,
	}
	if jpxIndex >= 0 {
		jpx, err := jpxStream(c.Extractor().R, stream, jpxIndex)
		if err != nil {
			return nil, err
		}
		data.jpx = jpx
		data.x = c.Extractor()
		data.maxBytes = limits.MaxImageBytes
	}
	return data, nil
}

// jpxStream returns a copy of stream whose filter chain is cut short
// before JPXDecode, which is at position jpxIndex.  The /Filter and
// /DecodeParms entries are sliced as they stand, so that reading does not
// depend on the version check filters apply when written.
func jpxStream(r pdf.Getter, stream *pdf.Stream, jpxIndex int) (*pdf.Stream, error) {
	res := *stream
	res.Dict = maps.Clone(stream.Dict)
	delete(res.Dict, "Filter")
	delete(res.Dict, "DecodeParms")
	if jpxIndex == 0 {
		return &res, nil
	}

	// jpxIndex > 0 implies that /Filter is an array
	filter, err := pdf.Resolve(r, stream.Dict["Filter"])
	if err != nil {
		return nil, err
	}
	parms, err := pdf.Resolve(r, stream.Dict["DecodeParms"])
	if err != nil {
		return nil, err
	}
	res.Dict["Filter"] = slices.Clone(filter.(pdf.Array)[:jpxIndex])
	if pa, ok := parms.(pdf.Array); ok && len(pa) > 0 {
		res.Dict["DecodeParms"] = slices.Clone(pa[:min(len(pa), jpxIndex)])
	}
	return &res, nil
}
