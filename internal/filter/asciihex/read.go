// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package asciihex

import (
	"bufio"
	"fmt"
	"io"
)

// Decode decodes data that has been encoded in ASCII hexadecimal form.
func Decode(r io.Reader) io.ReadCloser {
	return &reader{r: bufio.NewReader(r)}
}

type reader struct {
	r   *bufio.Reader
	err error
}

func (r *reader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}

	readHigh := false
	var low byte
readLoop:
	for n < len(p) {
		c, err := r.r.ReadByte()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			r.err = err
			break readLoop
		}

		var b byte

		switch c {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			b = c - '0'
		case 'A', 'B', 'C', 'D', 'E', 'F':
			b = c - 'A' + 10
		case 'a', 'b', 'c', 'd', 'e', 'f':
			b = c - 'a' + 10

		case 0, 9, 10, 12, 13, 32: // white-space
			continue readLoop

		case '>': // end of data
			if readHigh {
				p[n] = low << 4
				n++
			}
			r.err = io.EOF
			break readLoop

		default:
			r.err = fmt.Errorf("invalid hex character: %c", c)
			break readLoop
		}

		if readHigh {
			p[n] = low<<4 | b
			n++
			readHigh = false
		} else {
			low = b
			readHigh = true
		}
	}

	return n, r.err
}

func (r *reader) Close() error {
	if r.err == nil || r.err == io.EOF {
		return nil
	}
	return r.err
}
