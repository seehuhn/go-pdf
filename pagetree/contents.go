// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pagetree

import (
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/page"
)

// ContentStream returns a reader over a page's raw content-stream bytes.
// If /Contents is an array of streams, the streams are concatenated with
// single newlines between them (PDF 32000-1 §7.8.2).
//
// The returned reader yields bytes only, with no parsing or operator-level
// processing.  To iterate operators, decode the page with [page.Decode]
// and call [page.Page.NewIter] instead.
func ContentStream(r pdf.Getter, pageDict pdf.Object) (io.ReadCloser, error) {
	c := pdf.NewCursor(r)
	dict, err := c.DictTyped(pageDict, "Page")
	if err != nil {
		return nil, err
	}

	segments, err := page.ExtractContents(c, dict["Contents"])
	if err != nil {
		return nil, err
	}
	return page.SegmentsReader(segments), nil
}
