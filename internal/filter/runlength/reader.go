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

package runlength

import (
	"bufio"
	"io"
)

// Decode returns a new ReadCloser which decodes data in run-length format.
func Decode(r io.Reader) io.ReadCloser {
	return &rlReader{br: bufio.NewReader(r)}
}

type rlReader struct {
	br      *bufio.Reader
	err     error
	literal bool
	count   int
	value   byte
}

// Read implements the io.Reader interface.
func (r *rlReader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}

	for len(p) > 0 {
		if r.count > 0 {
			count := min(r.count, len(p))
			if r.literal {
				read, err := io.ReadFull(r.br, p[:count])
				n += read
				r.count -= read
				p = p[read:]
				if err != nil {
					r.err = err
					return n, err
				}
			} else {
				for i := range count {
					p[i] = r.value
				}
				n += count
				r.count -= count
				p = p[count:]
			}
			continue
		}

		length, err := r.br.ReadByte()
		if err != nil {
			if err == io.EOF && n > 0 {
				err = nil
			}
			r.err = err
			return n, err
		}

		switch {
		case length == 128:
			r.err = io.EOF
			return n, io.EOF

		case length < 128:
			r.count = int(length) + 1 // 1, ..., 128
			r.literal = true

		default: // length > 128
			r.count = 257 - int(length) // 2, ..., 128
			b, err := r.br.ReadByte()
			if err != nil {
				r.err = err
				return n, err
			}
			r.literal = false
			r.value = b
		}
	}

	return n, nil
}

// Close is a no-op.
func (r *rlReader) Close() error {
	return nil
}
