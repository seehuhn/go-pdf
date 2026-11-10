// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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

		// leave space for '\n'
		chunkSize := min((cap(w.buf)-len(w.buf)-1)/2, len(p))

		for i := range chunkSize {
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
