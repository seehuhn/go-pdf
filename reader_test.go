package pdflib

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestReadHeaderVersion(t *testing.T) {
	r := &Reader{
		r: strings.NewReader("%PDF-1.7\n"),
	}
	version, err := r.readHeaderVersion()
	if err != nil {
		t.Errorf("unexpected error %q", err)
	}
	if version != V1_7 {
		t.Errorf("wrong version: expected %d, got %d", V1_7, version)
	}

	for _, in := range []string{"", "%PEF-1.7\n", "%PDF-0.1\n"} {
		r = &Reader{
			r: strings.NewReader(in),
		}
		_, err = r.readHeaderVersion()
		if err == nil {
			t.Errorf("%q: missing error", in)
		}
	}

	for _, in := range []string{"%PDF-1.9\n", "%PDF-1.50\n"} {
		r = &Reader{
			r: strings.NewReader(in),
		}
		_, err = r.readHeaderVersion()
		if !errors.Is(err, errVersion) {
			t.Errorf("%q: wrong error %q", in, err)
		}
	}
}

func TestRealFile(t *testing.T) {
	name := "PDF32000_2008.pdf"
	// name := "example.pdf"
	fd, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := fd.Stat()
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewReader(fd, fi.Size())
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}
