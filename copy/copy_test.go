package main

import (
	"log"
	"os"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestIt(t *testing.T) {
	in, err := os.Open("simple.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		log.Fatal(err)
	}
	r, err := pdf.NewReader(in, fi.Size(), nil)
	if err != nil {
		log.Fatal(err)
	}

	out, err := os.Create("out.pdf")
	if err != nil {
		log.Fatal(err)
	}
	w, err := pdf.NewWriter(out, r.HeaderVersion)

	trans := &walker{
		trans: map[pdf.Reference]*pdf.Reference{},
		r:     r,
		w:     w,
	}
	obj, err := trans.Transfer(r.Trailer)
	if err != nil {
		log.Fatal(err)
	}

	trailer := obj.(pdf.Dict)
	err = w.Close(trailer["Root"].(*pdf.Reference), trailer["Info"].(*pdf.Reference))
	if err != nil {
		log.Fatal(err)
	}
}
