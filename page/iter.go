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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
)

// pageIter is the single-use [content.Iter] returned by [Page.NewIter].
//
// All segments are concatenated into one byte stream (with a newline
// inserted between them to enforce the inter-segment token boundary
// that PDF 32000-1 §7.8.2 requires) and parsed by a single
// [content.Scanner].  Operator args and any composite token state
// thus flow naturally across segment boundaries.
type pageIter struct {
	parts []content.Segment
	inner content.Iter
}

func (pi *pageIter) All() iter.Seq2[content.OpName, []pdf.Object] {
	open := func() (io.ReadCloser, error) {
		return &contentReader{parts: pi.parts}, nil
	}
	pi.inner = content.NewScanner(open).NewIter()
	return pi.inner.All()
}

func (pi *pageIter) Err() error {
	if pi.inner == nil {
		return nil
	}
	return pi.inner.Err()
}
