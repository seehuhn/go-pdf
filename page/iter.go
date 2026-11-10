// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
	parts []Segment
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
