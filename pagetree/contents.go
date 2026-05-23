// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	dict, err := pdf.GetDictTyped(r, pageDict, "Page")
	if err != nil {
		return nil, err
	}

	contents, err := pdf.Resolve(r, dict["Contents"])
	if err != nil {
		return nil, err
	}

	segments, err := page.ExtractContents(r, contents)
	if err != nil {
		return nil, err
	}
	return page.SegmentsReader(segments), nil
}
