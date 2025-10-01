package runlength

import (
	"io"
)

// Encode returns a new WriteCloser which encodes data in run-length format.
// The returned WriteCloser must be closed to flush all data.
func Encode(w io.WriteCloser) io.WriteCloser {
	return &rlWriter{w: w}
}

type rlWriter struct {
	w           io.WriteCloser
	buf         [129]byte
	used        int
	repeatCount int
	repeatVal   byte
}

// Write implements the io.Writer interface.
func (w *rlWriter) Write(p []byte) (n int, err error) {
	for n < len(p) {
		b := p[n]
		if w.repeatCount > 0 {
			if b == w.repeatVal && w.repeatCount < 128 {
				w.repeatCount++
				n++
				continue
			}

			err = w.flushRepeat()
			if err != nil {
				return n, err
			}
		}

		w.buf[1+w.used] = b
		w.used++
		n++

		if w.used >= 3 {
			idx := 1 + w.used - 3
			if w.buf[idx] == w.buf[idx+1] && w.buf[idx+1] == w.buf[idx+2] {
				literalCount := w.used - 3
				if literalCount > 0 {
					err = w.flushLiteral(literalCount)
					if err != nil {
						return n, err
					}
				}
				w.repeatCount = 3
				w.repeatVal = w.buf[idx]
				w.used = 0
				continue
			}
		}

		if w.used == 128 {
			err = w.flushLiteral(128)
			if err != nil {
				return n, err
			}
		}
	}

	return n, nil
}

func (w *rlWriter) flushLiteral(count int) error {
	w.buf[0] = byte(count - 1)
	_, err := w.w.Write(w.buf[0 : count+1])
	w.used = 0
	return err
}

func (w *rlWriter) flushRepeat() error {
	w.buf[0] = byte(257 - w.repeatCount)
	w.buf[1] = w.repeatVal
	_, err := w.w.Write(w.buf[0:2])
	w.repeatCount = 0
	return err
}

// Close flushes the remaining bytes and writes the EOD marker.
// It also closes the underlying writer.
func (w *rlWriter) Close() error {
	if w.repeatCount > 0 {
		err := w.flushRepeat()
		if err != nil {
			return err
		}
	}

	if w.used > 0 {
		err := w.flushLiteral(w.used)
		if err != nil {
			return err
		}
	}

	w.buf[0] = 128
	_, err := w.w.Write(w.buf[0:1])
	if err != nil {
		return err
	}

	return w.w.Close()
}
