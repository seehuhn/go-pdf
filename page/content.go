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
)

// RawBytes returns a reader over the page's raw content-stream bytes.
// Segments are concatenated with a single newline between them.  The
// returned reader yields bytes only, with no parsing or operator-level
// processing — see [Page.NewIter] for a tokenised view that threads
// scanner state across segment boundaries.  The caller is responsible
// for closing the returned [io.ReadCloser].
//
// The error return matches the Stream interface and is always nil; the
// underlying reader opens segments lazily.
func (p *Page) RawBytes() (io.ReadCloser, error) {
	return SegmentsReader(p.Contents), nil
}

// SegmentsReader returns a reader over the concatenated raw bytes of the
// given content-stream segments.  Segments are joined with a single
// newline (PDF 32000-1 §7.8.2).  The caller is responsible for closing
// the returned [io.ReadCloser].
func SegmentsReader(segments []Segment) io.ReadCloser {
	return &contentReader{parts: segments}
}

// A contentReader concatenates the raw bytes of all segments,
// separated by single newline bytes.  Per the library's permissive-reader
// policy, a segment whose [Segment.RawBytes] returns a malformed
// error is silently skipped — its absence shows up as a gap in the
// content stream rather than as a read failure.
type contentReader struct {
	parts []Segment
	idx   int           // index of next segment to open
	cur   io.ReadCloser // current segment reader, nil when between segments
	sep   bool          // true when a newline separator is pending
}

func (r *contentReader) Read(p []byte) (n int, err error) {
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
					// permissive-reader policy: skip the broken
					// segment and keep going.  The separator before
					// this segment, if any, was already emitted by
					// the sep branch above; no fresh one is needed.
					continue
				}
				return n, openErr
			}
			r.cur = rc
		}

		// read from the current segment
		nn, err := r.cur.Read(p[n:])
		n += nn
		if err == io.EOF {
			r.cur.Close()
			r.cur = nil
			// insert a separator before the next segment
			if r.idx < len(r.parts) {
				r.sep = true
			} else {
				return n, io.EOF
			}
		} else if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (r *contentReader) Close() error {
	if r.cur != nil {
		err := r.cur.Close()
		r.cur = nil
		return err
	}
	return nil
}
