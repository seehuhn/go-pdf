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
	start, err := r.findXRef()
	if err != nil {
		t.Error(err)
	}
	if start != 9 {
		t.Errorf("wrong xref start, expected 9 but got %d", start)
	}
}

func TestLastOccurence(t *testing.T) {
	buf := make([]byte, 2048)
	pat := "ABC"
	copy(buf[1023:], pat)

	r := &Reader{
		size: int64(len(buf)),
		r:    bytes.NewReader(buf),
	}
	pos, err := r.lastOccurence(pat)
	if err != nil {
		t.Fatal(err)
	}
	if pos != 1023 {
		t.Errorf("found wrong position: expected 1023, got %d", pos)
	}
}
