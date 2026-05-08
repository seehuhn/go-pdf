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
	"iter"
	"slices"
	"sync"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
)

// contentSegment is a lazily-decoded page content stream segment read from
// a PDF file. It implements [content.Stream].
//
// When a page has multiple content streams (a Contents array), each element
// becomes one contentSegment. The prev pointer chains segments so that
// trailing args (operator arguments split across stream boundaries) carry
// over to the next segment.
type contentSegment struct {
	prev   *contentSegment
	stream *pdf.Stream
	getter pdf.Getter

	res *content.Resources

	mu           sync.Mutex
	trailingArgs []pdf.Object
	decoded      bool
	decodeErr    error
}

var _ content.Stream = (*contentSegment)(nil)

// NewIter creates a new iterator for this content segment.
func (seg *contentSegment) NewIter() content.Iter {
	return &segmentIter{seg: seg}
}

// ensureDecoded scans the segment (and all predecessors) to compute
// trailingArgs. This is needed when a later segment requires the
// initial args from this one.
func (seg *contentSegment) ensureDecoded() error {
	seg.mu.Lock()
	defer seg.mu.Unlock()

	if seg.decoded {
		return seg.decodeErr
	}
	seg.decoded = true

	// ensure predecessor is decoded first
	var initArgs []pdf.Object
	if seg.prev != nil {
		if err := seg.prev.ensureDecoded(); err != nil {
			seg.decodeErr = err
			return err
		}
		seg.prev.mu.Lock()
		initArgs = slices.Clone(seg.prev.trailingArgs)
		seg.prev.mu.Unlock()
	}

	r, err := seg.rawReader()
	if err != nil {
		seg.decodeErr = err
		return err
	}
	defer r.Close()

	// scan the stream using a PageScanner to compute trailing args
	ps := content.NewPageScanner(pdf.GetVersion(seg.getter), seg.res)
	if len(initArgs) > 0 {
		ps.SetInitialArgs(initArgs)
	}
	// consume all operators (discarding them) to advance the scanner's state
	ps.ScanReader(r, func(content.OpName, []pdf.Object) bool {
		return true
	})
	seg.trailingArgs = ps.TrailingArgs()
	if psErr := ps.Err(); psErr != nil {
		seg.decodeErr = psErr
		return psErr
	}
	return nil
}

// rawReader returns a reader for the decoded stream bytes.
func (seg *contentSegment) rawReader() (io.ReadCloser, error) {
	return pdf.DecodeStream(seg.getter, nil, seg.stream, 0)
}

// Embed writes the content segment to a PDF file as a stream object.
func (seg *contentSegment) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	return embedContentStream(e, seg)
}

// segmentIter is a single-use [content.Iter] for a contentSegment.
type segmentIter struct {
	seg *contentSegment
	err error
}

// All returns an iterator over the operators in this segment.
// Trailing args from the previous segment are prepended to the first
// operator.
func (si *segmentIter) All() iter.Seq2[content.OpName, []pdf.Object] {
	return func(yield func(content.OpName, []pdf.Object) bool) {
		seg := si.seg

		// get initial args from predecessor
		var initArgs []pdf.Object
		if seg.prev != nil {
			if err := seg.prev.ensureDecoded(); err != nil {
				si.err = err
				return
			}
			seg.prev.mu.Lock()
			initArgs = slices.Clone(seg.prev.trailingArgs)
			seg.prev.mu.Unlock()
		}

		r, err := seg.rawReader()
		if err != nil {
			si.err = err
			return
		}
		defer r.Close()

		ps := content.NewPageScanner(pdf.GetVersion(seg.getter), seg.res)
		if len(initArgs) > 0 {
			ps.SetInitialArgs(initArgs)
		}
		exhausted := ps.ScanReader(r, yield)
		seg.mu.Lock()
		if psErr := ps.Err(); psErr != nil {
			si.err = psErr
		} else if exhausted && !seg.decoded {
			seg.decoded = true
			seg.trailingArgs = ps.TrailingArgs()
		}
		seg.mu.Unlock()
	}
}

// Err returns any IO error encountered during iteration.
func (si *segmentIter) Err() error {
	return si.err
}

// ClosingOperators returns nil because content segments are parts of a
// combined page stream whose closing operators are handled at the page level.
func (si *segmentIter) ClosingOperators() []content.OpName {
	return nil
}
