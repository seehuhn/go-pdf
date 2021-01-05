package fuzzing

import (
	"bytes"
	"io"
	"io/ioutil"

	"seehuhn.de/go/pdf"
)

// Fuzz is the entrance point for github.com/dvyukov/go-fuzz
func Fuzz(data []byte) int {
	buf := bytes.NewReader(data)
	r, err := pdf.NewReader(buf, buf.Size(), nil)
	if err != nil {
		return 0
	}

	seen := make(map[pdf.Reference]bool)
	err = r.Walk(r.Trailer, seen, func(o pdf.Object) error {
		if stream, ok := o.(*pdf.Stream); ok {
			_, err := io.Copy(ioutil.Discard, stream.R)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return 0
	}
	return 1
}
