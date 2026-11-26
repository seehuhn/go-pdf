// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package page

import (
	"io"

	"seehuhn.de/go/pdf"
)

// RawBytes returns a reader over the decoded bytes of the page's content
// streams.  Segments are concatenated with a single newline between them.  The
// returned reader yields bytes only, with no parsing or operator-level
// processing — see [Page.NewIter] for a tokenised view that threads
// scanner state across segment boundaries.  The caller is responsible
// for closing the returned [io.ReadCloser].
//
// A segment whose data cannot be decoded completely contributes the bytes
// decoded before the problem, and reading continues with the next segment.
// Use [SegmentsReader] with the page's Contents to learn whether this
// happened.
//
// The error return matches the Stream interface and is always nil; the
// underlying reader opens segments lazily.
func (p *Page) RawBytes() (io.ReadCloser, error) {
	return SegmentsReader(p.Contents), nil
}

// SegmentsReader returns a reader over the concatenated decoded bytes of
// the given content-stream segments.  Segments are joined with a single
// newline (PDF 32000-1 §7.8.2).
//
// A segment whose data cannot be decoded completely, for example because it
// uses an unknown filter or holds corrupt compressed data, contributes the
// bytes decoded before the problem, and reading continues with the next
// segment.  [ContentReader.Incomplete] tells whether this happened.  The
// caller is responsible for closing the reader.
func SegmentsReader(segments []Segment) *ContentReader {
	return &ContentReader{parts: segments}
}

// A ContentReader reads the decoded content of a list of segments, see
// [SegmentsReader].
type ContentReader struct {
	parts []Segment
	idx   int           // index of next segment to open
	cur   io.ReadCloser // current segment reader, nil when between segments
	sep   bool          // true when a newline separator is pending

	incomplete bool
}

// Incomplete reports whether a segment which the reader has reached so far
// could not be decoded completely.  The bytes read then lack the undecoded
// part of that segment.  Once Read has returned [io.EOF], the result covers
// all segments.
func (r *ContentReader) Incomplete() bool {
	return r.incomplete
}

// Read implements [io.Reader].  Segments which cannot be decoded completely
// are handled as described for [SegmentsReader]; other errors, such as
// read errors of the underlying file, are returned.
func (r *ContentReader) Read(p []byte) (n int, err error) {
	for n < len(p) {
		// emit a newline separator between segments
		if r.sep {
			p[n] = '\n'
			n++
			r.sep = false
			continue
		}

		// open the next segment if no current reader
		if r.cur == nil {
			if r.idx >= len(r.parts) {
				return n, io.EOF
			}
			rc, openErr := r.parts[r.idx].RawBytes()
			r.idx++
			if openErr != nil {
				if pdf.IsMalformed(openErr) {
					// skip the broken segment; the separator before
					// it, if any, has already been emitted
					r.incomplete = true
					continue
				}
				return n, openErr
			}
			r.cur = rc
		}

		// read from the current segment
		nn, err := r.cur.Read(p[n:])
		n += nn
		switch {
		case err == nil:
			// keep reading from this segment
		case err == io.EOF || pdf.IsMalformed(err):
			// the segment ended, or its filter data is corrupt or
			// truncated: keep the bytes decoded so far and move on to the
			// next segment
			if err != io.EOF {
				r.incomplete = true
			}
			r.cur.Close()
			r.cur = nil
			// insert a separator before the next segment
			if r.idx < len(r.parts) {
				r.sep = true
			} else {
				return n, io.EOF
			}
		default:
			return n, err
		}
	}
	return n, nil
}

// Close closes the reader of the current segment.
func (r *ContentReader) Close() error {
	if r.cur != nil {
		err := r.cur.Close()
		r.cur = nil
		return err
	}
	return nil
}
