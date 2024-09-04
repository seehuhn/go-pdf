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
	"io"
)

func Encode(w io.WriteCloser, width int) io.WriteCloser {
	return &writer{
		w:   w,
		buf: make([]byte, 0, width+1),
	}
}

type writer struct {
	w   io.WriteCloser
	buf []byte
	err error
}

func (w *writer) Write(p []byte) (n int, err error) {
	for len(p) > 0 && w.err == nil {
		if len(w.buf)+3 > cap(w.buf) { // space for "xx\n"
			if len(w.buf) > 0 {
				w.buf = append(w.buf, '\n')
			}
			w.flush()
			if w.err != nil {
				break
			}
		}

		chunkSize := (cap(w.buf) - len(w.buf) - 1) / 2 // leave space for '\n'
		if chunkSize > len(p) {
			chunkSize = len(p)
		}

		for i := 0; i < chunkSize; i++ {
			w.buf = append(w.buf, alphabet[p[i]>>4], alphabet[p[i]&0x0f])
		}
		p = p[chunkSize:]
		n += chunkSize
	}
	return n, w.err
}

func (w *writer) Close() error {
	if w.err != nil {
		return w.err
	}

	if len(w.buf)+2 > cap(w.buf) { // space for ">\n"
		w.buf = append(w.buf, '\n')
		w.flush()
		if w.err != nil {
			return w.err
		}
	}

	w.buf = append(w.buf, '>')
	w.flush()
	if w.err != nil {
		return w.err
	}

	w.err = w.w.Close()
	return w.err
}

func (w *writer) flush() {
	if len(w.buf) == 0 || w.err != nil {
		return
	}
	_, w.err = w.w.Write(w.buf)
	w.buf = w.buf[:0]
}

const alphabet = "0123456789abcdef"
