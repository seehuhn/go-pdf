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
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
)

// ContentStream creates a reader for the content stream(s) of a PDF page.
// The pageDict is the dictionary of the page object.
// It handles cases where Contents is a single stream or an array of streams.
// Decoded streams are concatenated, separated by newline characters.
// If the /Contents entry is absent, null, or an empty array, an empty reader is returned.
func ContentStream(r pdf.Getter, pageDict pdf.Object) (io.Reader, error) {
	dict, err := pdf.GetDictTyped(r, pageDict, "Page")
	if err != nil {
		return nil, fmt.Errorf("getting page dictionary: %w", err)
	}

	contents, err := pdf.Resolve(r, dict["Contents"])
	if err != nil {
		return nil, err
	} else if contents == nil {
		// empty page
		return eofReader{}, nil
	}

	var a pdf.Array
	switch contents := contents.(type) {
	case pdf.Array:
		if len(contents) == 0 {
			return eofReader{}, nil
		}
		a = contents
	default:
		a = pdf.Array{contents}
	}

	return &contentsReader{
		r: r,
		a: a,
	}, nil
}

type eofReader struct{}

func (e eofReader) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

// a contentsReader allows to read from the contents stream of a PDF page.
type contentsReader struct {
	// r is the PDF reader that contains the data.
	r pdf.Getter

	// a is the array of stream objects to read from. The reader returns the
	// contents of each stream in order, separated by newline characters.
	a pdf.Array

	// err is the first error encountered, or io.EOF once all streams have been exhausted.
	err error

	// current is the reader for the currently active stream, if any.
	current io.ReadCloser

	needNewline bool // true if a newline should be prepended before the next read.
}

func (cr *contentsReader) Read(p []byte) (int, error) {
	if cr.err != nil {
		return 0, cr.err
	}

	if len(p) == 0 {
		return 0, nil
	}

	r, err := cr.getReader()
	if err != nil {
		cr.err = err
		return 0, err
	}

	extra := 0
	if cr.needNewline {
		p[0] = '\n'
		extra = 1
		p = p[1:]
		cr.needNewline = false

		if len(p) == 0 {
			return extra, nil
		}
	}

	n, err := r.Read(p)
	if err == io.EOF {
		closeErr := cr.current.Close()
		if closeErr != nil {
			cr.err = closeErr
			return extra + n, closeErr
		}
		cr.current = nil

		if len(cr.a) > 0 {
			// continue with the next stream
			err = nil
			cr.needNewline = true
		} else {
			// end of all streams
			cr.err = io.EOF
			if extra+n > 0 {
				err = nil
			}
		}
	} else if err != nil {
		cr.err = err
		cr.current.Close() // ignore errors, because we already have an error
		cr.current = nil
	}

	return extra + n, err
}

func (cr *contentsReader) getReader() (r io.ReadCloser, err error) {
	if cr.current != nil {
		return cr.current, nil
	}

	var stm *pdf.Stream
	for {
		if len(cr.a) == 0 {
			return nil, io.EOF
		}

		obj := cr.a[0]
		cr.a = cr.a[1:]

		stm, err = pdf.GetStream(cr.r, obj)
		if err != nil {
			return nil, err
		}
		if stm != nil {
			break
		}
	}

	r, err = pdf.DecodeStream(cr.r, stm, 0)
	if err != nil {
		return nil, err
	}
	cr.current = r
	return r, nil
}
