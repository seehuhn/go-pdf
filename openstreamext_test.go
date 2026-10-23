// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package pdf_test

import (
	"bytes"
	"compress/zlib"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func zlibBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// encodeWith applies the given filters (outermost first) to data.
func encodeWith(t *testing.T, data []byte, filters ...pdf.Filter) []byte {
	t.Helper()
	var buf bytes.Buffer
	var wc io.WriteCloser = nopCloser{&buf}
	var err error
	for _, f := range filters {
		wc, err = f.Encode(pdf.V2_0, wc)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := wc.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := wc.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

func TestOpenStreamExistingFilters(t *testing.T) {
	data := []byte("some stream data, some stream data, some stream data")

	// pre-encoded data for a /FlateDecode chain with predictor parameters
	predFilter := pdf.FilterFlate{Predictor: 12, Columns: 4}
	predData := encodeWith(t, data, predFilter)

	type testCase struct {
		name       string
		dict       pdf.Dict
		filters    []pdf.Filter
		pre        []byte // bytes written, already encoded with the dict's filters
		wantFilter pdf.Object
		wantParms  pdf.Object
	}
	cases := []testCase{
		{
			name:       "name plus one",
			dict:       pdf.Dict{"Filter": pdf.Name("FlateDecode")},
			filters:    []pdf.Filter{pdf.FilterASCII85{}},
			pre:        zlibBytes(t, data),
			wantFilter: pdf.Array{pdf.Name("ASCII85Decode"), pdf.Name("FlateDecode")},
		},
		{
			name:    "name plus two",
			dict:    pdf.Dict{"Filter": pdf.Name("FlateDecode")},
			filters: []pdf.Filter{pdf.FilterASCIIHex{}, pdf.FilterASCII85{}},
			pre:     zlibBytes(t, data),
			wantFilter: pdf.Array{
				pdf.Name("ASCIIHexDecode"), pdf.Name("ASCII85Decode"), pdf.Name("FlateDecode"),
			},
		},
		{
			name: "array plus one",
			dict: pdf.Dict{"Filter": pdf.Array{
				pdf.Name("ASCII85Decode"), pdf.Name("FlateDecode"),
			}},
			filters: []pdf.Filter{pdf.FilterASCIIHex{}},
			pre:     encodeWith(t, zlibBytes(t, data), pdf.FilterASCII85{}),
			wantFilter: pdf.Array{
				pdf.Name("ASCIIHexDecode"), pdf.Name("ASCII85Decode"), pdf.Name("FlateDecode"),
			},
		},
		{
			name: "name with parms",
			dict: pdf.Dict{
				"Filter": pdf.Name("FlateDecode"),
				"DecodeParms": pdf.Dict{
					"Predictor": pdf.Integer(12), "Columns": pdf.Integer(4),
				},
			},
			filters:    []pdf.Filter{pdf.FilterASCII85{}},
			pre:        predData,
			wantFilter: pdf.Array{pdf.Name("ASCII85Decode"), pdf.Name("FlateDecode")},
			wantParms: pdf.Array{nil, pdf.Dict{
				"Predictor": pdf.Integer(12), "Columns": pdf.Integer(4),
			}},
		},
		{
			// a single dict next to a two-element chain belongs to no
			// filter, and is dropped as the reader drops it
			name: "array with misshaped parms",
			dict: pdf.Dict{
				"Filter": pdf.Array{pdf.Name("ASCII85Decode"), pdf.Name("FlateDecode")},
				"DecodeParms": pdf.Dict{
					"Predictor": pdf.Integer(12), "Columns": pdf.Integer(4),
				},
			},
			filters: []pdf.Filter{pdf.FilterASCIIHex{}},
			pre:     encodeWith(t, zlibBytes(t, data), pdf.FilterASCII85{}),
			wantFilter: pdf.Array{
				pdf.Name("ASCIIHexDecode"), pdf.Name("ASCII85Decode"), pdf.Name("FlateDecode"),
			},
		},
		{
			name:       "no existing filter",
			dict:       pdf.Dict{},
			filters:    []pdf.Filter{pdf.FilterASCIIHex{}, pdf.FilterASCII85{}},
			pre:        data,
			wantFilter: pdf.Array{pdf.Name("ASCIIHexDecode"), pdf.Name("ASCII85Decode")},
		},
		{
			name: "existing crypt",
			dict: pdf.Dict{"Filter": pdf.Array{
				pdf.Name("Crypt"), pdf.Name("FlateDecode"),
			}},
			filters: []pdf.Filter{pdf.FilterASCII85{}},
			pre:     zlibBytes(t, data),
			wantFilter: pdf.Array{
				pdf.Name("Crypt"), pdf.Name("ASCII85Decode"), pdf.Name("FlateDecode"),
			},
		},
		{
			name: "existing identity crypt with parms",
			dict: pdf.Dict{
				"Filter": pdf.Array{pdf.Name("Crypt"), pdf.Name("FlateDecode")},
				"DecodeParms": pdf.Array{
					pdf.Dict{"Name": pdf.Name("Identity")}, nil,
				},
			},
			filters: []pdf.Filter{pdf.FilterASCII85{}},
			pre:     zlibBytes(t, data),
			wantFilter: pdf.Array{
				pdf.Name("Crypt"), pdf.Name("ASCII85Decode"), pdf.Name("FlateDecode"),
			},
			wantParms: pdf.Array{
				pdf.Dict{"Name": pdf.Name("Identity")}, nil, nil,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			ref := w.Alloc()
			body, err := w.OpenStream(ref, c.dict, c.filters...)
			if err != nil {
				t.Fatal(err)
			}
			// the writer applies the new filters, so we write the data
			// which is encoded with the dict's own filters
			if _, err := body.Write(c.pre); err != nil {
				t.Fatal(err)
			}
			if err := body.Close(); err != nil {
				t.Fatal(err)
			}

			obj, err := pdf.Resolve(w, ref)
			if err != nil {
				t.Fatal(err)
			}
			stm := obj.(*pdf.Stream)
			if diff := cmp.Diff(c.wantFilter, stm.Dict["Filter"]); diff != "" {
				t.Errorf("wrong /Filter (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(c.wantParms, stm.Dict["DecodeParms"]); diff != "" {
				t.Errorf("wrong /DecodeParms (-want +got):\n%s", diff)
			}

			r, err := pdf.DecodeStream(w, nil, stm)
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, data) {
				t.Errorf("round trip failed: got %q, want %q", got, data)
			}
		})
	}
}

func TestOpenStreamCryptErrors(t *testing.T) {
	t.Run("named crypt in unencrypted file", func(t *testing.T) {
		// a named crypt filter has no meaning without encryption: a
		// dict entry is dropped, an explicit filter is refused
		w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		dict := pdf.Dict{
			"Filter": pdf.Array{pdf.Name("Crypt"), pdf.Name("FlateDecode")},
			"DecodeParms": pdf.Array{
				pdf.Dict{"Name": pdf.Name("StdCF")}, nil,
			},
		}
		ref := w.Alloc()
		s, err := w.OpenStream(ref, dict)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		stm, err := pdf.NewCursor(w).Stream(ref)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(pdf.Name("FlateDecode"), stm.Dict["Filter"]); diff != "" {
			t.Errorf("wrong /Filter (-want +got):\n%s", diff)
		}
		if _, ok := stm.Dict["DecodeParms"]; ok {
			t.Errorf("unexpected /DecodeParms %v", stm.Dict["DecodeParms"])
		}

		_, err = w.OpenStream(w.Alloc(), nil, pdf.FilterCryptStandard{})
		if err == nil {
			t.Error("expected error")
		}
	})
	t.Run("crypt name not a name", func(t *testing.T) {
		for _, opt := range []*pdf.WriterOptions{nil, {UserPassword: "u"}} {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, opt)
			dict := pdf.Dict{
				"Filter":      pdf.Name("Crypt"),
				"DecodeParms": pdf.Dict{"Name": pdf.Integer(1)},
			}
			_, err := w.OpenStream(w.Alloc(), dict)
			if err == nil {
				t.Error("expected error")
			}
		}
	})
	t.Run("duplicate crypt", func(t *testing.T) {
		w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		dict := pdf.Dict{
			"Filter": pdf.Array{pdf.Name("Crypt"), pdf.Name("FlateDecode")},
		}
		_, err := w.OpenStream(w.Alloc(), dict, pdf.FilterCryptIdentity{})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// TestOpenStreamDuplicateFilter verifies that OpenStream refuses to
// compose a chain which names a filter twice, while a dict whose chain
// already repeats a filter can still be written back as it is.
func TestOpenStreamDuplicateFilter(t *testing.T) {
	t.Run("among new filters", func(t *testing.T) {
		w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		_, err := w.OpenStream(w.Alloc(), nil, pdf.FilterFlate{}, pdf.FilterASCIIHex{}, pdf.FilterFlate{})
		if err == nil {
			t.Error("expected error")
		}
	})
	t.Run("new and existing", func(t *testing.T) {
		w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		dict := pdf.Dict{"Filter": pdf.Name("FlateDecode")}
		_, err := w.OpenStream(w.Alloc(), dict, pdf.FilterFlate{})
		if err == nil {
			t.Error("expected error")
		}
	})
	t.Run("existing only", func(t *testing.T) {
		w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		dict := pdf.Dict{"Filter": pdf.Array{pdf.Name("ASCIIHexDecode"), pdf.Name("ASCIIHexDecode")}}
		stm, err := w.OpenStream(w.Alloc(), dict)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stm.Write([]byte("3E3E>")); err != nil {
			t.Fatal(err)
		}
		if err := stm.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

// TestOpenStreamDictCryptAddFilter checks that a filter can be added to a
// stream whose dict already selects a named crypt filter.
func TestOpenStreamDictCryptAddFilter(t *testing.T) {
	data := []byte("some stream data, some stream data, some stream data")
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, &pdf.WriterOptions{UserPassword: "u"})
	dict := pdf.Dict{
		"Filter":      pdf.Name("Crypt"),
		"DecodeParms": pdf.Dict{"Name": pdf.Name("StdCF")},
	}
	ref := w.Alloc()
	body, err := w.OpenStream(ref, dict, pdf.FilterFlate{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := body.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}

	obj, err := pdf.Resolve(w, ref)
	if err != nil {
		t.Fatal(err)
	}
	stm := obj.(*pdf.Stream)
	wantFilter := pdf.Array{pdf.Name("Crypt"), pdf.Name("FlateDecode")}
	if diff := cmp.Diff(wantFilter, stm.Dict["Filter"]); diff != "" {
		t.Errorf("wrong /Filter (-want +got):\n%s", diff)
	}
	wantParms := pdf.Array{pdf.Dict{"Name": pdf.Name("StdCF")}, nil}
	if diff := cmp.Diff(wantParms, stm.Dict["DecodeParms"]); diff != "" {
		t.Errorf("wrong /DecodeParms (-want +got):\n%s", diff)
	}

	r, err := pdf.DecodeStream(w, nil, stm)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("round trip failed: got %q, want %q", got, data)
	}
}
