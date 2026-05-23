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

package page

import (
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
)

var _ content.Segment = (*Source)(nil)

// Source is a lazy file-backed page content-stream segment.
//
// When a page is decoded from a PDF file, each of its /Contents stream
// objects becomes one *Source in Page.Contents.  Sharing the same *Source
// value across multiple pages (or among multiple positions in the same
// Page.Contents list) causes the underlying PDF stream object to be
// referenced once and shared, via pointer-identity deduplication in
// [pdf.ResourceManager].
//
// A Source represents one byte-segment of a page's logical content
// stream; operators, paths and text objects may straddle the boundary
// to a neighbouring segment.  Iteration is therefore only meaningful in
// the unified view provided by [Page.NewIter], which threads scanner
// state across all segments.
type Source struct {
	stream *pdf.Stream
	getter pdf.Getter
}

// RawBytes returns a reader over the segment's decoded content-stream
// bytes.  Used by [Page.NewIter] to feed Source segments into the
// page's combined content-stream scanner.
func (s *Source) RawBytes() (io.ReadCloser, error) {
	return pdf.DecodeStream(s.getter, nil, s.stream, 0)
}

// Embed writes the segment's decoded bytes verbatim to a new PDF stream
// object.  No fix-ups are applied at embed time — bytes are preserved
// as faithfully as the underlying stream filters allow.  Repeated embeds
// of the same *Source value dedup to a single output object via
// [pdf.ResourceManager].
func (s *Source) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	r, err := s.RawBytes()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	ref := e.Out().Alloc()
	var filters []pdf.Filter
	if !e.Out().GetOptions().HasAny(pdf.OptPretty) {
		filters = append(filters, pdf.FilterCompress{})
	}
	stm, err := e.Out().OpenStream(ref, nil, filters...)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(stm, r); err != nil {
		stm.Close()
		return nil, err
	}
	if err := stm.Close(); err != nil {
		return nil, err
	}
	return ref, nil
}
