// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package page_test

import (
	"bytes"
	"compress/zlib"
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/page"
)

// rawSource writes a stream with the given dictionary and undecoded data,
// and returns it as a segment.
func rawSource(t *testing.T, dict pdf.Dict, data []byte) page.Segment {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	ref := w.Alloc()
	stm, err := w.OpenStream(ref, dict)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	x := pdf.NewExtractor(w)
	s, err := pdf.CursorAt(x, nil).Stream(ref)
	if err != nil {
		t.Fatal(err)
	}
	return page.NewSource(s, x)
}

func TestSegmentsReaderIncomplete(t *testing.T) {
	const head = "0 0 m\n"
	const tail = "10 10 l S\n"

	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	zw.Write([]byte(head))
	zw.Flush()
	cut := zbuf.Len()
	zw.Write([]byte(tail))
	zw.Close()
	truncated := zbuf.Bytes()[:cut+(zbuf.Len()-cut)/2]

	ops := &content.Operators{Ops: []content.Operator{{Name: content.OpFill}}}

	cases := []struct {
		name       string
		seg        page.Segment
		want       string
		incomplete bool
	}{
		{"plain", rawSource(t, pdf.Dict{}, []byte(head+tail)), head + tail, false},
		{"truncated Flate", rawSource(t, pdf.Dict{"Filter": pdf.Name("FlateDecode")}, truncated), head, true},
		{"unknown filter", rawSource(t, pdf.Dict{"Filter": pdf.Name("NoSuchDecode")}, []byte(head)), "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := page.SegmentsReader([]page.Segment{c.seg, ops})
			if r.Incomplete() {
				t.Error("incomplete before reading")
			}
			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}

			// the following segment is read in full
			next, err := ops.RawBytes()
			if err != nil {
				t.Fatal(err)
			}
			nextData, err := io.ReadAll(next)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(got, []byte(c.want)) || !bytes.HasSuffix(got, nextData) {
				t.Errorf("got %q, want %q followed by the next segment", got, c.want)
			}
			if r.Incomplete() != c.incomplete {
				t.Errorf("Incomplete() = %t, want %t", r.Incomplete(), c.incomplete)
			}
		})
	}
}
