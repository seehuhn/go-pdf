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

	seen := make(map[pdf.Reference]pdf.Object)
	_, err = r.Walk(pdf.Array{pdf.Struct(r.Catalog), pdf.Struct(r.Info)},
		seen, func(o pdf.Object) (pdf.Object, error) {
			if stream, ok := o.(*pdf.Stream); ok {
				_, err := io.Copy(ioutil.Discard, stream.R)
				if err != nil {
					return nil, err
				}
			}
			return nil, nil
		}, nil)

	if err != nil {
		return 0
	}
	return 1
}
