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
	"iter"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
)

// combinedPageStream wraps multiple content.Stream parts into a single
// content.Stream, concatenating their operators.
// When all parts are *contentSegment (the normal case for pages read from
// file), a single PageScanner is used so that scanner state (current
// object type, graphics state stack, trailing args) carries across stream
// boundaries.
type combinedPageStream struct {
	parts []content.Stream
	err   error
}

var _ content.Stream = (*combinedPageStream)(nil)

// All returns an iterator over all operators from all parts.
func (c *combinedPageStream) All() iter.Seq2[content.OpName, []pdf.Object] {
	// check whether all parts are contentSegments so we can use a
	// single PageScanner with shared state
	segments := make([]*contentSegment, len(c.parts))
	for i, p := range c.parts {
		seg, ok := p.(*contentSegment)
		if !ok {
			return c.allFallback()
		}
		segments[i] = seg
	}
	return c.allScanner(segments)
}

// allScanner iterates using a single PageScanner across all raw readers,
// so that path construction state carries across stream boundaries.
func (c *combinedPageStream) allScanner(segments []*contentSegment) iter.Seq2[content.OpName, []pdf.Object] {
	return func(yield func(content.OpName, []pdf.Object) bool) {
		if len(segments) == 0 {
			return
		}
		ps := content.NewPageScanner(pdf.GetVersion(segments[0].getter), segments[0].res)
		for _, seg := range segments {
			r, err := seg.rawReader()
			if err != nil {
				c.err = err
				return
			}
			exhausted := ps.ScanReader(r, yield)
			r.Close()
			if !exhausted {
				return
			}
		}
		for _, name := range ps.ClosingOps() {
			if !yield(name, nil) {
				return
			}
		}
		if psErr := ps.Err(); psErr != nil {
			c.err = psErr
		}
	}
}

// allFallback iterates each part independently (used when parts are
// not all *contentSegment).
func (c *combinedPageStream) allFallback() iter.Seq2[content.OpName, []pdf.Object] {
	return func(yield func(content.OpName, []pdf.Object) bool) {
		for _, part := range c.parts {
			for name, args := range part.All() {
				if !yield(name, args) {
					return
				}
			}
			if pErr := part.Err(); pErr != nil {
				c.err = pErr
				return
			}
		}
	}
}

// Err returns any IO error encountered during iteration.
func (c *combinedPageStream) Err() error {
	return c.err
}

// embedContentStream writes a content.Stream to a PDF file as a stream object.
func embedContentStream(e *pdf.EmbedHelper, s content.Stream) (pdf.Native, error) {
	ref := e.Alloc()

	var filters []pdf.Filter
	if !e.Out().GetOptions().HasAny(pdf.OptPretty) {
		filters = append(filters, pdf.FilterCompress{})
	}

	stm, err := e.Out().OpenStream(ref, nil, filters...)
	if err != nil {
		return nil, err
	}

	for name, args := range s.All() {
		op := content.Operator{Name: name, Args: args}
		if err := content.WriteOperator(stm, op); err != nil {
			stm.Close()
			return nil, err
		}
	}
	if sErr := s.Err(); sErr != nil {
		stm.Close()
		return nil, sErr
	}

	if err := stm.Close(); err != nil {
		return nil, err
	}

	return ref, nil
}

// embedPageContent embeds a content.Stream into a PDF file.
// If the stream implements [pdf.Embedder] (e.g. *contentSegment),
// dedup is based on pointer identity.
// Other types are wrapped, so each call produces a new stream object.
func embedPageContent(rm *pdf.ResourceManager, s content.Stream) (pdf.Native, error) {
	if emb, ok := s.(pdf.Embedder); ok {
		return rm.Embed(emb)
	}
	return rm.Embed(&contentStreamWrapper{s})
}

// contentStreamWrapper wraps a content.Stream as a pdf.Embedder
// for stream types that don't implement Embedder themselves.
type contentStreamWrapper struct {
	stream content.Stream
}

func (c *contentStreamWrapper) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	return embedContentStream(e, c.stream)
}

var _ pdf.Embedder = (*contentStreamWrapper)(nil)
