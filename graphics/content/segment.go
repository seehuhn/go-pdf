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

package content

import (
	"io"

	"seehuhn.de/go/pdf"
)

// Segment is one element of a page's /Contents array.
//
// A PDF page's content can be split across multiple stream objects which
// the viewer parses as if their decoded bytes were concatenated (PDF
// 32000-1 §7.8.2). Each stream object corresponds to one Segment.
// Iterating a single Segment in isolation is therefore not generally
// meaningful: an operator, a path, or a text object may straddle the
// boundary to the next segment. Use [seehuhn.de/go/pdf/page.Page.NewIter]
// to obtain a unified iterator that threads scanner state across all
// segments of a page.
//
// Implementations:
//   - [*Operators] is an in-memory segment built from an [Operator] slice.
//   - [seehuhn.de/go/pdf/page.Source] is a file-backed segment that
//     copies its bytes verbatim when re-embedded.
//
// Two segments compare as equal via [pdf.ResourceManager] when they are
// the same Go pointer; this gives pointer-identity deduplication when
// the same segment is shared across multiple pages (for example, a
// footer drawn on every page).
type Segment interface {
	pdf.Embedder

	// RawBytes returns a reader over the segment's content-stream bytes.
	// The caller is responsible for closing the returned [io.ReadCloser].
	//
	// The bytes returned here are the same bytes that [pdf.Embedder.Embed]
	// would write into the segment's PDF stream object, except for
	// stream-object framing and filters. They are intended for feeding
	// into a [Scanner] when iterating a page's combined content stream.
	RawBytes() (io.ReadCloser, error)
}
