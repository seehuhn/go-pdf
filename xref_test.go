package pdflib

import (
	"bytes"
	"strings"
	"testing"
)

func TestFindXref(t *testing.T) {
	in := "%PDF-1.7\nhello\nstartxref\n9\n%%EOF"
	r := &Reader{
		size: int64(len(in)),
		r:    strings.NewReader(in),
	}
	start, stop, err := r.findXRef()
	if err != nil {
		t.Error(err)
	}
	buf := []byte(in)[start:stop]
	if !bytes.Equal(buf, []byte("hello\n")) {
		t.Errorf("wrong xref data, expected %q but got %q",
			"hello\n", string(buf))
	}
}
