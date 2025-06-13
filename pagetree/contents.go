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
)

// ContentStream creates a reader for the content stream of a PDF page. The
// pageDict is the dictionary of the page object. If Contents is an array of
// streams, the streams are concatenated, separated by newline characters.
func ContentStream(r pdf.Getter, pageDict pdf.Object) (io.Reader, error) {
	dict, err := pdf.GetDictTyped(r, pageDict, "Page")
	if err != nil {
		return nil, err
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

	return &contentReader{
		r: r,
		a: a,
	}, nil
}

type eofReader struct{}

func (eofReader) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

// contentReader reads from the content stream of a PDF page.
type contentReader struct {
	r           pdf.Getter    // PDF reader containing the data
	a           pdf.Array     // stream objects to read from
	err         error         // first error encountered, or io.EOF when exhausted
	current     io.ReadCloser // reader for the currently active stream
	needNewline bool          // true if a newline should be prepended before the next read
}

func (cr *contentReader) Read(p []byte) (int, error) {
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
		if closeErr := cr.current.Close(); closeErr != nil {
			cr.err = closeErr
			return extra + n, closeErr
		}
		cr.current = nil

		if len(cr.a) > 0 {
			cr.needNewline = true
			err = nil
		} else {
			cr.err = io.EOF
			if extra+n > 0 {
				err = nil
			}
		}
	} else if err != nil {
		cr.err = err
		cr.current.Close() // ignore errors, since we already have an error
		cr.current = nil
	}

	return extra + n, err
}

func (cr *contentReader) getReader() (io.ReadCloser, error) {
	if cr.current != nil {
		return cr.current, nil
	}

	for len(cr.a) > 0 {
		ref := cr.a[0]
		cr.a = cr.a[1:]

		stm, err := pdf.GetStream(cr.r, ref)
		if err != nil {
			return nil, err
		}
		if stm != nil {
			r, err := pdf.DecodeStream(cr.r, stm, 0)
			if err != nil {
				return nil, err
			}
			cr.current = r
			return r, nil
		}
	}

	return nil, io.EOF
}
