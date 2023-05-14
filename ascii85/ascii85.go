// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package ascii85

import (
	"errors"
	"io"
)

// Encode returns a new WriteCloser which encodes data in ASCII85 format.
// The returned WriteCloser] must be closed to flush all data.
func Encode(w io.WriteCloser, width int) (io.WriteCloser, error) {
	return &ascii85Writer{
		w:   w,
		buf: make([]byte, 0, width+1),
	}, nil
}

// Decode returns a new Reader which decodes data in ASCII85 format.
func Decode(r io.Reader) (io.Reader, error) {
	return &ascii85Reader{r: r}, nil
}

type ascii85Reader struct {
	r              io.Reader
	immediateError error
	delayedError   error
	buf            [512]byte
	outbuf         [4]byte
	leftover       []byte
	pos, nbuf      int
	v              uint32
	k              int
	isEnd          bool
}

// Read implements the [io.Reader] interface.
func (r *ascii85Reader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.immediateError != nil {
		return 0, r.immediateError
	}

	if len(r.leftover) > 0 {
		n = copy(p, r.leftover)
		r.leftover = r.leftover[n:]
	}

	for n < len(p) {
		// get the next input byte
		for r.pos == r.nbuf && r.delayedError == nil {
			r.nbuf, r.delayedError = r.r.Read(r.buf[:])
			r.pos = 0

			if r.delayedError == io.EOF {
				r.delayedError = io.ErrUnexpectedEOF
			}
		}
		if r.pos == r.nbuf {
			r.immediateError = r.delayedError
			return n, r.immediateError
		}

		for r.pos < r.nbuf {
			c := r.buf[r.pos]
			r.pos++

			// "~" can only be the first part of the end marker "~>"
			if r.isEnd {
				if c == '>' {
					r.immediateError = io.EOF
				} else {
					r.immediateError = errors.New("invalid end marker in ASCII85 stream")
				}
				return n, r.immediateError
			}

			if c >= '!' && c < '!'+85 {
				r.v = r.v*85 + uint32(c-'!')
				r.k++
			} else if r.k == 0 && c == 'z' {
				r.v = 0
				r.k = 5
			} else if isSpace(c) {
				continue
			} else if c == '~' {
				switch r.k {
				case 0:
					// pass
				case 1:
					r.immediateError = errors.New("unexpected end marker in ASCII85 stream")
					return n, r.immediateError
				default:
					for i := r.k; i < 5; i++ {
						r.v = r.v*85 + 84
					}
					r.outbuf[0] = byte(r.v >> 24)
					r.outbuf[1] = byte(r.v >> 16)
					r.outbuf[2] = byte(r.v >> 8)
					r.outbuf[3] = byte(r.v)
					l := copy(p[n:], r.outbuf[:r.k-1])
					n += l
					if l < r.k-1 {
						r.leftover = r.outbuf[l : r.k-1]
					}
					r.k = 0
				}
				r.isEnd = true
				continue
			} else {
				r.immediateError = errors.New("invalid character in ASCII85 stream")
				return n, r.immediateError
			}

			if r.k == 5 {
				r.outbuf[0] = byte(r.v >> 24)
				r.outbuf[1] = byte(r.v >> 16)
				r.outbuf[2] = byte(r.v >> 8)
				r.outbuf[3] = byte(r.v)
				r.k = 0
				r.v = 0

				l := copy(p[n:], r.outbuf[:])
				n += l
				if l < 4 {
					r.leftover = r.outbuf[l:]
				}
				break // need to re-check whether the buffer is full
			}
		}
	}
	return n, r.immediateError
}

type ascii85Writer struct {
	w   io.WriteCloser
	buf []byte
	v   uint32
	k   int
}

// Write implements the [io.Writer] interface.
func (w *ascii85Writer) Write(p []byte) (n int, err error) {
	for n, b := range p {
		w.v = w.v<<8 | uint32(b)
		w.k++
		if w.k == 4 {
			if cap(w.buf) < len(w.buf)+8 { // space for "xxxxx~>\n"
				err = w.flush()
				if err != nil {
					return n, err
				}
			}

			v := w.v
			if v == 0 {
				w.buf = append(w.buf, 'z')
			} else {
				c4 := byte(v%85) + '!'
				v /= 85
				c3 := byte(v%85) + '!'
				v /= 85
				c2 := byte(v%85) + '!'
				v /= 85
				c1 := byte(v%85) + '!'
				v /= 85
				c0 := byte(v%85) + '!'
				w.buf = append(w.buf, c0, c1, c2, c3, c4)
			}

			w.v = 0
			w.k = 0
		}
	}
	return len(p), nil
}

// Close flushes the remaining bytes and writes the end marker.
// It also closes the underlying writer.
// This implements the [io.Closer] interface.
func (w *ascii85Writer) Close() error {
	if w.k != 0 {
		v := w.v << ((4 - w.k) * 8)
		var c [5]byte
		for i := 4; i >= 0; i-- {
			c[i] = byte(v%85) + '!'
			v /= 85
		}
		w.buf = append(w.buf, c[:w.k+1]...)
		w.v = 0
		w.k = 0
	}
	w.buf = append(w.buf, '~', '>')
	err := w.flush()
	if err != nil {
		return err
	}
	return w.w.Close()
}

func (w *ascii85Writer) flush() error {
	w.buf = append(w.buf, '\n')
	_, err := w.w.Write(w.buf)
	if err != nil {
		return err
	}
	w.buf = w.buf[:0]
	return nil
}

func isSpace(c byte) bool {
	switch c {
	case 0, 9, 10, 12, 13, 32:
		return true
	default:
		return false
	}
}
