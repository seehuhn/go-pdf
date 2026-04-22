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
}

var _ content.Stream = (*combinedPageStream)(nil)

// NewIter creates a new iterator over all combined parts.
func (c *combinedPageStream) NewIter() content.Iter {
	return &combinedIter{parts: c.parts}
}

// combinedIter is a single-use [content.Iter] for a combinedPageStream.
type combinedIter struct {
	parts []content.Stream
	err   error
}

// All returns an iterator over all operators from all parts.
func (ci *combinedIter) All() iter.Seq2[content.OpName, []pdf.Object] {
	// check whether all parts are contentSegments so we can use a
	// single PageScanner with shared state
	segments := make([]*contentSegment, len(ci.parts))
	for i, p := range ci.parts {
		seg, ok := p.(*contentSegment)
		if !ok {
			return ci.allFallback()
		}
		segments[i] = seg
	}
	return ci.allScanner(segments)
}

// allScanner iterates using a single PageScanner across all raw readers,
// so that path construction state carries across stream boundaries.
func (ci *combinedIter) allScanner(segments []*contentSegment) iter.Seq2[content.OpName, []pdf.Object] {
	return func(yield func(content.OpName, []pdf.Object) bool) {
		if len(segments) == 0 {
			return
		}
		ps := content.NewPageScanner(pdf.GetVersion(segments[0].getter), segments[0].res)
		for _, seg := range segments {
			r, err := seg.rawReader()
			if err != nil {
				ci.err = err
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
			ci.err = psErr
		}
	}
}

// allFallback iterates each part independently (used when parts are
// not all *contentSegment).
func (ci *combinedIter) allFallback() iter.Seq2[content.OpName, []pdf.Object] {
	return func(yield func(content.OpName, []pdf.Object) bool) {
		for _, part := range ci.parts {
			it := part.NewIter()
			for name, args := range it.All() {
				if !yield(name, args) {
					return
				}
			}
			if pErr := it.Err(); pErr != nil {
				ci.err = pErr
				return
			}
			for _, name := range it.ClosingOperators() {
				if !yield(name, nil) {
					return
				}
			}
		}
	}
}

// Err returns any IO error encountered during iteration.
func (ci *combinedIter) Err() error {
	return ci.err
}

// ClosingOperators returns nil because the combined scanner already
// yields closing operators at the end of [combinedIter.All].
func (ci *combinedIter) ClosingOperators() []content.OpName {
	return nil
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

	it := s.NewIter()
	for name, args := range it.All() {
		op := content.Operator{Name: name, Args: args}
		if err := content.WriteOperator(stm, op); err != nil {
			stm.Close()
			return nil, err
		}
	}
	if sErr := it.Err(); sErr != nil {
		stm.Close()
		return nil, sErr
	}
	for _, name := range it.ClosingOperators() {
		if err := content.WriteOperator(stm, content.Operator{Name: name}); err != nil {
			stm.Close()
			return nil, err
		}
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
