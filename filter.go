package pdflib

import (
	"compress/zlib"
	"fmt"
	"io"
)

func applyFilter(r io.Reader, name Object, param Object) io.Reader {
	n, ok := name.(Name)
	if !ok {
		return &errorReader{
			fmt.Errorf("invalid filter description %s", format(name))}
	}
	switch string(n) {
	case "FlateDecode":
		zr, err := zlib.NewReader(r)
		if err != nil {
			return &errorReader{err}
		}
		return zr
	default:
		return &errorReader{fmt.Errorf("unsupported filter %q", n)}
	}
}

type errorReader struct {
	err error
}

func (e *errorReader) Read([]byte) (int, error) {
	return 0, e.err
}
