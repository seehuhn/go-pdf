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
// given PDF stream object.  The extractor is used to resolve indirect
// references when the segment's bytes are read or re-embedded.
func NewSource(stream *pdf.Stream, x *pdf.Extractor) *Source {
	return &Source{stream: stream, x: x}
}

// ExtractContents builds the [Page.Contents] segment list from a /Contents
// object, which may be a Reference, a direct stream, or an array of either.
// A stream reached through a Reference is decoded via [pdf.Decode], through
// c, so it carries provenance (see [Source]).  An array entry that is itself
// a direct stream is malformed (streams must be indirect); such entries are
// read permissively but carry no provenance.  A nil or unrecognised object
// yields a nil segment list.
func ExtractContents(c pdf.Cursor, contents pdf.Object) ([]Segment, error) {
	resolved, err := c.Resolve(contents)
	if err != nil {
		return nil, err
	}
	switch v := resolved.(type) {
	case *pdf.Stream:
		if ref, ok := contents.(pdf.Reference); ok {
			src, err := pdf.Decode(c, ref, decodeSource)
			if err != nil {
				return nil, err
			}
			if src == nil {
				return nil, nil
			}
			return []Segment{src}, nil
		}
		// malformed: a direct stream where an indirect one is required
		return []Segment{NewSource(v, c.Extractor())}, nil
	case pdf.Array:
		segments := make([]Segment, 0, len(v))
		for _, item := range v {
			if ref, ok := item.(pdf.Reference); ok {
				src, err := pdf.Decode(c, ref, decodeSource)
				if err != nil {
					return nil, err
				}
				if src != nil {
					segments = append(segments, src)
				}
				continue
			}
			// malformed: a direct stream where an indirect one is required
			stm, err := c.Stream(item)
			if err != nil {
				return nil, err
			}
			if stm == nil {
				continue
			}
			segments = append(segments, NewSource(stm, c.Extractor()))
		}
		return segments, nil
	}
	return nil, nil
}

// decodeSource decodes a single content-stream object into a *Source.  Used
// through [pdf.Decode] so an indirect stream carries provenance.
func decodeSource(c pdf.Cursor, obj pdf.Object, _ bool) (*Source, error) {
	stm, err := c.Stream(obj)
	if err != nil {
		return nil, err
	}
	if stm == nil {
		return nil, nil
	}
	return NewSource(stm, c.Extractor()), nil
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
	x      *pdf.Extractor
}

// RawBytes returns a reader over the segment's decoded content-stream
// bytes.  Used by [Page.NewIter] to feed Source segments into the
// page's combined content-stream scanner.
func (s *Source) RawBytes() (io.ReadCloser, error) {
	return pdf.CursorAt(s.x, nil).StreamReader(s.stream)
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

	ref := e.AllocSelf()
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
