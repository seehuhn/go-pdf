package fuzzing

import (
	"bytes"
	"io"
	"io/ioutil"

	"seehuhn.de/go/pdflib"
)

// Fuzz is the entrance point for github.com/dvyukov/go-fuzz
func Fuzz(data []byte) int {
	buf := bytes.NewReader(data)
	r, err := pdflib.NewReader(buf, buf.Size(), nil)
	if err != nil {
		return 0
	}

	seen := make(map[pdflib.Reference]bool)
	err = r.Walk(r.Trailer, seen, func(o pdflib.Object) error {
		if stream, ok := o.(*pdflib.Stream); ok {
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
