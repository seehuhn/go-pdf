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
	"bytes"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
)

var (
	_ Segment = (*Source)(nil)
	_ Segment = (*content.Operators)(nil)
)

// NewSource creates a file-backed content stream segment wrapping the
// given PDF stream object.  The getter is used to resolve indirect
// references when the segment's bytes are read or re-embedded.
func NewSource(stream *pdf.Stream, getter pdf.Getter) *Source {
	return &Source{stream: stream, getter: getter}
}

// ExtractContents builds the [Page.Contents] segment list from a resolved
// /Contents object.  The argument must already have any top-level indirect
// reference resolved (see [pdf.Resolve]); array entries are resolved
// internally.  A nil or unrecognised object yields a nil segment list,
// matching the permissive-reader policy.
func ExtractContents(r pdf.Getter, contents pdf.Object) ([]Segment, error) {
	switch c := contents.(type) {
	case *pdf.Stream:
		return []Segment{NewSource(c, r)}, nil
	case pdf.Array:
		segments := make([]Segment, 0, len(c))
		for _, item := range c {
			stm, err := pdf.GetStream(r, item)
			if err != nil {
				return nil, err
			}
			if stm == nil {
				continue
			}
			segments = append(segments, NewSource(stm, r))
		}
		return segments, nil
	}
	return nil, nil
}

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
// as faithfully as the underlying stream filters allow.  A segment whose
// filters cannot be decoded is written as an empty stream, matching the
// way a reader skips it.  Repeated embeds of the same *Source value dedup
// to a single output object via [pdf.ResourceManager].
func (s *Source) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	r, err := s.RawBytes()
	if err != nil {
		if !pdf.IsMalformed(err) {
			return nil, err
		}
		// The segment uses a filter the library cannot decode (e.g. an
		// unknown or unimplemented filter).  A reader treats such a segment
		// as empty (see contentReader.Read), and emitting the original
		// filter name would produce an invalid PDF, so write an empty
		// content stream to keep the page valid and round-tripping.
		r = io.NopCloser(bytes.NewReader(nil))
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
		// a corrupt or truncated filter stream yields a malformed error
		// part-way through; keep the bytes decoded so far, matching the way
		// a reader uses the decoded prefix (see contentReader.Read)
		if !pdf.IsMalformed(err) {
			stm.Close()
			return nil, err
		}
	}
	if err := stm.Close(); err != nil {
		return nil, err
	}
	return ref, nil
}
